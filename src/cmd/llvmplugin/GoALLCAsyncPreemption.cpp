// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "llvm/ADT/DenseMap.h"
#include "llvm/ADT/STLExtras.h"
#include "llvm/ADT/SmallPtrSet.h"
#include "llvm/ADT/SmallVector.h"
#include "llvm/ADT/StringRef.h"
#include "llvm/Analysis/OptimizationRemarkEmitter.h"
#include "llvm/Analysis/PostDominators.h"
#include "llvm/Analysis/ValueTracking.h"
#include "llvm/CodeGen/GoCallingConv.h"
#include "llvm/CodeGen/GoObjAsyncPreemption.h"
#include "llvm/CodeGen/MachineBasicBlock.h"
#include "llvm/CodeGen/MachineFunction.h"
#include "llvm/CodeGen/MachineInstr.h"
#include "llvm/CodeGen/MachineMemOperand.h"
#include "llvm/CodeGen/MachineOperand.h"
#include "llvm/CodeGen/TargetInstrInfo.h"
#include "llvm/CodeGen/TargetRegisterInfo.h"
#include "llvm/IR/CFG.h"
#include "llvm/IR/DiagnosticInfo.h"
#include "llvm/IR/Dominators.h"
#include "llvm/IR/Function.h"
#include "llvm/IR/GlobalValue.h"
#include "llvm/IR/Instructions.h"
#include "llvm/IR/IntrinsicInst.h"
#include "llvm/Target/TargetMachine.h"

using namespace llvm;

#define DEBUG_TYPE "goallc-async-preemption"

namespace {

constexpr StringLiteral GoALLCGCName = "goallc";
constexpr StringLiteral WriteBarrierFlagName = "runtime.writeBarrier";

bool hasLogicalGoSymbolName(StringRef Encoded, StringRef Logical) {
  if (!Encoded.consume_front(Logical))
    return false;
  return Encoded.empty() || Encoded.starts_with("<");
}

bool isWriteBarrierFlag(const Value *V) {
  if (!V)
    return false;
  V = getUnderlyingObject(V->stripPointerCasts());
  const auto *GV = dyn_cast<GlobalValue>(V);
  return GV && hasLogicalGoSymbolName(GV->getName(), WriteBarrierFlagName);
}

bool isWriteBarrierFlagLoad(const Instruction &I) {
  const auto *Load = dyn_cast<LoadInst>(&I);
  return Load && isWriteBarrierFlag(Load->getPointerOperand());
}

bool isWriteBarrierOperation(const CallBase &Call) {
  if (const auto *II = dyn_cast<IntrinsicInst>(&Call))
    if (II->getIntrinsicID() == Intrinsic::go_gc_write_barrier)
      return true;
  const Function *Callee = Call.getCalledFunction();
  if (!Callee)
    return false;
  StringRef Name = Callee->getName();
  return hasLogicalGoSymbolName(Name, "runtime.wbMove") ||
         hasLogicalGoSymbolName(Name, "runtime.wbZero");
}

struct WriteBarrierRegions {
  // Entire machine blocks mapped to these IR blocks are unsafe. This includes
  // the enabled/disabled arms and their common block through the raw heap
  // write. The common block is deliberately block-granular because LLVM IR
  // has no Go WBend marker after optimization.
  SmallPtrSet<const BasicBlock *, 16> UnsafeBlocks;
  // In a decision block, only PCs after the write-barrier flag load are
  // semantically unsafe. Machine lowering locates that load where possible.
  SmallPtrSet<const BasicBlock *, 8> DecisionBlocks;
};

bool collectWriteBarrierRegions(const Function &F, WriteBarrierRegions &Out) {
  SmallVector<const CondBrInst *, 8> Checks;
  SmallVector<const CallBase *, 8> Operations;
  SmallPtrSet<const Value *, 32> FlagDependent;
  SmallVector<const Value *, 16> Worklist;
  for (const BasicBlock &BB : F) {
    for (const Instruction &I : BB) {
      if (isWriteBarrierFlagLoad(I)) {
        FlagDependent.insert(&I);
        Worklist.push_back(&I);
      }
      if (const auto *Call = dyn_cast<CallBase>(&I);
          Call && isWriteBarrierOperation(*Call))
        Operations.push_back(Call);
    }
  }

  // Compute the same operand-graph reachability once, in the reverse
  // direction. Shared conditions and cyclic PHIs no longer repeat a DFS for
  // every branch. Only instructions and constant expressions participate,
  // matching the original dependency walk.
  while (!Worklist.empty()) {
    const Value *V = Worklist.pop_back_val();
    for (const User *U : V->users()) {
      if (!isa<Instruction, ConstantExpr>(U) ||
          !FlagDependent.insert(U).second)
        continue;
      Worklist.push_back(U);
    }
  }
  for (const BasicBlock &BB : F)
    if (const auto *Branch = dyn_cast<CondBrInst>(BB.getTerminator()))
      if (FlagDependent.contains(Branch->getCondition()))
        Checks.push_back(Branch);

  if (Checks.empty() && Operations.empty())
    return true;
  if (Checks.empty() || Operations.empty())
    return false;

  DominatorTree DT(const_cast<Function &>(F));
  PostDominatorTree PDT(const_cast<Function &>(F));
  SmallPtrSet<const CondBrInst *, 8> UsedChecks;

  for (const CallBase *Operation : Operations) {
    const BasicBlock *OperationBB = Operation->getParent();
    const CondBrInst *Best = nullptr;
    unsigned BestLevel = 0;
    for (const CondBrInst *Check : Checks) {
      const BasicBlock *CheckBB = Check->getParent();
      if (!DT.dominates(CheckBB, OperationBB))
        continue;

      unsigned DominatingSuccessors = 0;
      for (unsigned I = 0; I != 2; ++I)
        DominatingSuccessors +=
            DT.dominates(Check->getSuccessor(I), OperationBB);
      if (DominatingSuccessors != 1)
        continue;

      const DomTreeNode *Node = DT.getNode(CheckBB);
      if (!Node)
        continue;
      unsigned Level = Node->getLevel();
      if (!Best || Level > BestLevel) {
        Best = Check;
        BestLevel = Level;
      }
    }
    if (!Best)
      return false;

    const BasicBlock *CheckBB = Best->getParent();
    const BasicBlock *Join = PDT.findNearestCommonDominator(
        Best->getSuccessor(0), Best->getSuccessor(1));
    if (!Join || !PDT.dominates(Join, CheckBB) ||
        !PDT.dominates(Join, OperationBB))
      return false;

    UsedChecks.insert(Best);
    Out.DecisionBlocks.insert(CheckBB);
    Out.UnsafeBlocks.insert(Join);
    for (const BasicBlock &BB : F) {
      if (&BB == CheckBB || &BB == Join)
        continue;
      if (DT.dominates(CheckBB, &BB) && PDT.dominates(Join, &BB))
        Out.UnsafeBlocks.insert(&BB);
    }
  }

  // A compiler-emitted flag decision with no recognized buffer/helper action
  // means optimization produced a protocol shape we do not understand. Do not
  // silently expose any PC in that function as an async safe point.
  return UsedChecks.size() == Checks.size();
}

void emitUnsafeFallbackRemark(const Function &F, StringRef Reason) {
  OptimizationRemarkEmitter ORE(&F);
  ORE.emit([&]() {
    return OptimizationRemarkMissed(DEBUG_TYPE, "UnsafeFallback", &F)
           << "kept function unsafe for asynchronous preemption: " << Reason;
  });
}

MCRegister findPhysicalRegister(const TargetRegisterInfo &TRI, StringRef Name) {
  for (unsigned Reg = 1; Reg != TRI.getNumRegs(); ++Reg)
    if (Name == TRI.getName(Reg))
      return MCRegister(Reg);
  return MCRegister();
}

bool machineOperandNames(const MachineOperand &MO, StringRef Logical) {
  if (MO.isGlobal())
    return hasLogicalGoSymbolName(MO.getGlobal()->getName(), Logical);
  if (MO.isSymbol())
    return hasLogicalGoSymbolName(MO.getSymbolName(), Logical);
  return false;
}

bool machineInstrNames(const MachineInstr &MI, StringRef Logical) {
  for (const MachineOperand &MO : MI.operands())
    if (machineOperandNames(MO, Logical))
      return true;
  return false;
}

bool machineInstrLoadsWriteBarrierFlag(const MachineInstr &MI) {
  if (!MI.mayLoad())
    return false;
  if (machineInstrNames(MI, WriteBarrierFlagName))
    return true;
  for (const MachineMemOperand *MMO : MI.memoperands())
    if (isWriteBarrierFlag(MMO->getValue()))
      return true;
  return false;
}

void markWholeMachineBlock(const MachineBasicBlock &MBB,
                           GoObjUnsafeInstructionMarker MarkUnsafe) {
  for (const MachineInstr &MI : MBB)
    if (!MI.isMetaInstruction())
      MarkUnsafe(MI);
}

void markWriteBarrierMachineRanges(const MachineFunction &MF,
                                   const WriteBarrierRegions &Regions,
                                   GoObjUnsafeInstructionMarker MarkUnsafe) {
  for (const MachineBasicBlock &MBB : MF) {
    const BasicBlock *IRBB = MBB.getBasicBlock();
    if (!IRBB)
      continue;
    if (Regions.UnsafeBlocks.contains(IRBB)) {
      markWholeMachineBlock(MBB, MarkUnsafe);
      continue;
    }
    if (!Regions.DecisionBlocks.contains(IRBB))
      continue;

    const MachineInstr *FlagLoad = nullptr;
    for (const MachineInstr &MI : MBB)
      if (machineInstrLoadsWriteBarrierFlag(MI)) {
        FlagLoad = &MI;
        break;
      }

    // If instruction selection obscured the load, keeping this small decision
    // block entirely unsafe is conservative without sacrificing the rest of
    // the function. Otherwise the unsafe interval starts at the instruction
    // immediately following the completed load.
    bool AfterFlagLoad = FlagLoad == nullptr;
    for (const MachineInstr &MI : MBB) {
      if (AfterFlagLoad && !MI.isMetaInstruction())
        MarkUnsafe(MI);
      if (&MI == FlagLoad)
        AfterFlagLoad = true;
    }
  }
}

bool isMorestackCall(const MachineInstr &MI) {
  if (!MI.isCall())
    return false;
  return machineInstrNames(MI, "runtime.morestack") ||
         machineInstrNames(MI, "runtime.morestack_noctxt") ||
         machineInstrNames(MI, "runtime.morestackc");
}

bool markMorestackRanges(const MachineFunction &MF,
                         GoObjUnsafeInstructionMarker MarkUnsafe) {
  SmallPtrSet<const MachineInstr *, 4> Calls;
  SmallPtrSet<const MachineBasicBlock *, 16> StackCheckBlocks;
  SmallVector<const MachineBasicBlock *, 8> Worklist;
  for (const MachineBasicBlock &MBB : MF)
    for (const MachineInstr &MI : MBB)
      if (isMorestackCall(MI)) {
        if (MBB.getBasicBlock())
          return false;
        Calls.insert(&MI);
        if (StackCheckBlocks.insert(&MBB).second)
          Worklist.push_back(&MBB);
      }

  while (!Worklist.empty()) {
    const MachineBasicBlock *MBB = Worklist.pop_back_val();
    auto Visit = [&](const MachineBasicBlock *Adjacent) {
      if (!Adjacent->getBasicBlock() &&
          StackCheckBlocks.insert(Adjacent).second)
        Worklist.push_back(Adjacent);
    };
    for (const MachineBasicBlock *Pred : MBB->predecessors())
      Visit(Pred);
    for (const MachineBasicBlock *Succ : MBB->successors())
      Visit(Succ);
  }

  for (const MachineBasicBlock *MBB : StackCheckBlocks) {
    const MachineInstr *LastCall = nullptr;
    for (const MachineInstr &MI : *MBB)
      if (Calls.contains(&MI))
        LastCall = &MI;
    if (!LastCall) {
      markWholeMachineBlock(*MBB, MarkUnsafe);
      continue;
    }
    for (const MachineInstr &MI : *MBB) {
      if (!MI.isMetaInstruction())
        MarkUnsafe(MI);
      if (&MI == LastCall)
        break;
    }
  }
  return true;
}

void markFrameTransitionRanges(const MachineFunction &MF,
                               GoObjUnsafeInstructionMarker MarkUnsafe) {
  // LLVM's prologue and epilogue lowering may update a Go frame in several
  // instructions. For example, AArch64 can save FP/LR below a not-yet-committed
  // future SP and only then move that value into SP. An asynchronous preemption
  // in the middle can use the same memory for its injected frames and corrupt
  // the saved linkage. The inverse teardown has the same transient mismatch
  // between the physical registers and the PCSP-described frame.
  //
  // FrameSetup and FrameDestroy are the target-independent MachineInstr
  // contract for these transactions. Keeping just these short target-lowered
  // ranges unsafe avoids constraining IR optimization or the ordinary body.
  for (const MachineBasicBlock &MBB : MF)
    for (const MachineInstr &MI : MBB)
      if (!MI.isMetaInstruction() && (MI.getFlag(MachineInstr::FrameSetup) ||
                                      MI.getFlag(MachineInstr::FrameDestroy)))
        MarkUnsafe(MI);
}

bool isEncodedRegisterRead(const MachineInstr &MI, const TargetInstrInfo &TII,
                           MCRegister Reg) {
  return TII.getName(MI.getOpcode()) == StringRef("READ_REGISTER_GPR64") &&
         MI.getNumOperands() > 1 && MI.getOperand(1).isImm() &&
         MI.getOperand(1).getImm() == Reg.id();
}

bool markAArch64RegTmpRanges(const MachineFunction &MF,
                             GoObjUnsafeInstructionMarker MarkUnsafe) {
  const TargetInstrInfo &TII = *MF.getSubtarget().getInstrInfo();
  const TargetRegisterInfo &TRI = *MF.getSubtarget().getRegisterInfo();
  MCRegister RegTmp = findPhysicalRegister(TRI, "X27");
  if (!RegTmp)
    return false;

  DenseMap<const MachineBasicBlock *, bool> UsesBeforeDef;
  DenseMap<const MachineBasicBlock *, bool> Defines;
  DenseMap<const MachineBasicBlock *, bool> LiveIn;
  DenseMap<const MachineBasicBlock *, bool> LiveOut;
  for (const MachineBasicBlock &MBB : MF) {
    bool SawDef = false;
    bool Use = false;
    for (const MachineInstr &MI : MBB) {
      if (MI.isMetaInstruction())
        continue;
      bool Reads = isEncodedRegisterRead(MI, TII, RegTmp) ||
                   MI.readsRegister(RegTmp, &TRI);
      bool Defs = MI.modifiesRegister(RegTmp, &TRI);
      if (Reads && !SawDef)
        Use = true;
      SawDef |= Defs;
    }
    UsesBeforeDef[&MBB] = Use;
    Defines[&MBB] = SawDef;
  }

  bool Changed;
  do {
    Changed = false;
    for (const MachineBasicBlock &MBB : reverse(MF)) {
      bool Out = false;
      for (const MachineBasicBlock *Succ : MBB.successors())
        Out |= LiveIn.lookup(Succ);
      bool In = UsesBeforeDef.lookup(&MBB) || (Out && !Defines.lookup(&MBB));
      if (LiveOut.lookup(&MBB) != Out || LiveIn.lookup(&MBB) != In) {
        LiveOut[&MBB] = Out;
        LiveIn[&MBB] = In;
        Changed = true;
      }
    }
  } while (Changed);

  for (const MachineBasicBlock &MBB : MF) {
    bool Live = LiveOut.lookup(&MBB);
    for (const MachineInstr &MI : reverse(MBB)) {
      if (MI.isMetaInstruction())
        continue;
      bool Reads = isEncodedRegisterRead(MI, TII, RegTmp) ||
                   MI.readsRegister(RegTmp, &TRI);
      bool Defs = MI.modifiesRegister(RegTmp, &TRI);
      bool LiveBefore = Reads || (Live && !Defs);
      if (LiveBefore || (Defs && TII.getInstSizeInBytes(MI) > 4))
        MarkUnsafe(MI);
      Live = LiveBefore;
    }
  }
  return true;
}

bool markInlineAsmRanges(const MachineFunction &MF, StringRef StackRegName,
                         GoObjUnsafeInstructionMarker MarkUnsafe) {
  const TargetRegisterInfo &TRI = *MF.getSubtarget().getRegisterInfo();
  MCRegister StackReg = findPhysicalRegister(TRI, StackRegName);
  if (!StackReg)
    return false;
  for (const MachineBasicBlock &MBB : MF)
    for (const MachineInstr &MI : MBB)
      if (MI.isInlineAsm()) {
        if (MI.modifiesRegister(StackReg, &TRI))
          return false;
        MarkUnsafe(MI);
      }
  return true;
}

bool hasWholeFunctionPolicy(const Function &F) {
  StringRef Name = F.getName();
  return F.hasFnAttribute(goabi::NoSplitAttr) ||
         F.hasFnAttribute(goabi::SystemStackAttr) ||
         Name.starts_with("runtime.") ||
         Name.starts_with("internal/runtime/") || Name.starts_with("reflect.");
}

GoObjAsyncPreemptionMode
describeAsyncPreemption(const MachineFunction &MF,
                        GoObjUnsafeInstructionMarker MarkUnsafe) {
  const Function &F = MF.getFunction();
  if (!MF.getTarget().getTargetTriple().isOSBinFormatGoObj() ||
      !goabi::isGoCallingConv(F.getCallingConv()) || !F.hasGC() ||
      F.getGC() != GoALLCGCName)
    return GoObjAsyncPreemptionMode::Unhandled;

  if (hasWholeFunctionPolicy(F))
    return GoObjAsyncPreemptionMode::WholeFunctionUnsafe;

  Triple::ArchType Arch = MF.getTarget().getTargetTriple().getArch();
  if (Arch != Triple::x86_64 && Arch != Triple::aarch64 &&
      Arch != Triple::aarch64_be) {
    emitUnsafeFallbackRemark(F, "unsupported target architecture");
    return GoObjAsyncPreemptionMode::WholeFunctionUnsafe;
  }

  WriteBarrierRegions Regions;
  if (!collectWriteBarrierRegions(F, Regions)) {
    emitUnsafeFallbackRemark(F, "unrecognized write-barrier protocol");
    return GoObjAsyncPreemptionMode::WholeFunctionUnsafe;
  }
  markWriteBarrierMachineRanges(MF, Regions, MarkUnsafe);

  if (!markMorestackRanges(MF, MarkUnsafe)) {
    emitUnsafeFallbackRemark(F, "unrecognized morestack control flow");
    return GoObjAsyncPreemptionMode::WholeFunctionUnsafe;
  }

  markFrameTransitionRanges(MF, MarkUnsafe);

  StringRef StackRegName = Arch == Triple::x86_64 ? "RSP" : "SP";
  if (!markInlineAsmRanges(MF, StackRegName, MarkUnsafe)) {
    emitUnsafeFallbackRemark(F, "inline assembly has unmodeled stack state");
    return GoObjAsyncPreemptionMode::WholeFunctionUnsafe;
  }

  if ((Arch == Triple::aarch64 || Arch == Triple::aarch64_be) &&
      !markAArch64RegTmpRanges(MF, MarkUnsafe)) {
    emitUnsafeFallbackRemark(F, "AArch64 REGTMP is unavailable");
    return GoObjAsyncPreemptionMode::WholeFunctionUnsafe;
  }

  return GoObjAsyncPreemptionMode::InstructionRanges;
}

RegisterGoObjAsyncPreemptionCallback
    RegisterGoALLCAsyncPreemption(describeAsyncPreemption);

} // namespace
