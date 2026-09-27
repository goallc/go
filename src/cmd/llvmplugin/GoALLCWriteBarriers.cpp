// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCWriteBarriers.h"
#include "llvm/ADT/SmallPtrSet.h"
#include "llvm/ADT/SmallVector.h"
#include "llvm/Analysis/AssumptionCache.h"
#include "llvm/Analysis/BasicAliasAnalysis.h"
#include "llvm/Analysis/MemoryBuiltins.h"
#include "llvm/Analysis/MemorySSA.h"
#include "llvm/Analysis/TargetLibraryInfo.h"
#include "llvm/Analysis/ValueTracking.h"
#include "llvm/IR/Dominators.h"
#include "llvm/IR/IRBuilder.h"
#include "llvm/IR/IntrinsicInst.h"
#include "llvm/IR/MDBuilder.h"
#include "llvm/IR/Module.h"
#include "llvm/Support/ErrorHandling.h"
#include "llvm/Transforms/Utils/BasicBlockUtils.h"

using namespace llvm;

namespace llvm::goallc {
namespace {
constexpr StringLiteral RecordName = "goallc.gc.write.record";
bool isWriteBarrierRecord(const CallInst *CI) {
  const Function *F = CI->getCalledFunction();
  return F && F->getName() == RecordName;
}

// Query the optimized IR before expanding any records or changing the CFG.
// Allocation attributes describe the initial contents, not the contents at an
// arbitrary later store: MemorySSA and AA must also rule out intervening
// writes.
void findZeroOldValues(Function &F, ArrayRef<CallInst *> Writes,
                       SmallPtrSetImpl<CallInst *> &ZeroOld) {
  const DataLayout &DL = F.getDataLayout();
  TargetLibraryInfoImpl TLII(F.getParent()->getTargetTriple());
  TargetLibraryInfo TLI(TLII, &F);
  AssumptionCache AC(F);
  DominatorTree DT(F);
  BasicAAResult BAA(DL, F, TLI, AC, &DT);
  AAResults AA(TLI);
  AA.addAAResult(BAA);
  MemorySSA MSSA(F, &AA, &DT);
  Type *PtrTy = PointerType::getUnqual(F.getContext());
  for (CallInst *CI : Writes) {
    Value *Dst = CI->getArgOperand(1);
    MemoryLocation Loc(Dst, LocationSize::precise(DL.getTypeStoreSize(PtrTy)));
    auto *Access = MSSA.getMemoryAccess(CI);
    if (!Access)
      continue;
    auto *Def = dyn_cast<MemoryDef>(MSSA.getWalker()->getClobberingMemoryAccess(
        Access->getDefiningAccess(), Loc));
    if (!Def || MSSA.isLiveOnEntryDef(Def))
      continue;
    Instruction *I = Def->getMemoryInst();
    if (I == getUnderlyingObject(Dst)) {
      if (Constant *Init = getInitialValueOfAllocation(I, &TLI, PtrTy))
        if (Init->isNullValue())
          ZeroOld.insert(CI);
    } else if (auto *Store = dyn_cast<StoreInst>(I)) {
      if (Store->isSimple() &&
          isa<ConstantPointerNull>(Store->getValueOperand()) &&
          MemoryLocation::get(Store).Size == Loc.Size &&
          AA.alias(MemoryLocation::get(Store), Loc) == AliasResult::MustAlias)
        ZeroOld.insert(CI);
    }
  }
}
} // namespace

void configureWriteBarrierRecords(Module &M) {
  Function *F = M.getFunction(RecordName);
  if (!F)
    return;
  LLVMContext &C = M.getContext();
  auto *Expected =
      FunctionType::get(Type::getVoidTy(C),
                        {PointerType::getUnqual(C), PointerType::getUnqual(C),
                         Type::getInt32Ty(C)},
                        false);
  if (!F->isDeclaration() || F->getFunctionType() != Expected)
    report_fatal_error("invalid Go write barrier record declaration");
  for (User *U : F->users()) {
    auto *CI = dyn_cast<CallInst>(U);
    if (!CI || CI->getCalledFunction() != F || !CI->getFunction()->hasGC() ||
        CI->getFunction()->getGC() != "goallc" || CI->isMustTailCall())
      report_fatal_error("invalid Go write barrier record use");
    auto *Flags = dyn_cast<ConstantInt>(CI->getArgOperand(2));
    if (Flags && Flags->getZExtValue() > 3)
      report_fatal_error("invalid Go write barrier record flags");
  }
  // The GC is non-moving. Recording neither publishes a pointer to Go code
  // nor changes user-visible memory. Reading the destination orders the record
  // before the ordinary store, including when the old-value read is omitted.
  // Inaccessible writes retain the GC side effect even if the store is dead.
  F->addFnAttr("gc-leaf-function");
  F->setMemoryEffects(MemoryEffects::argMemOnly(ModRefInfo::Ref) |
                      MemoryEffects::inaccessibleMemOnly(ModRefInfo::ModRef));
  for (auto A : {Attribute::NoUnwind, Attribute::WillReturn, Attribute::NoFree,
                 Attribute::NoSync, Attribute::NoCallback})
    F->addFnAttr(A);
  F->addParamAttr(0, Attribute::ReadNone);
  F->addParamAttr(1, Attribute::ReadOnly);
  for (unsigned I : {0u, 1u})
    F->addParamAttr(I, Attribute::getWithCaptureInfo(C, CaptureInfo::none()));
}

// Expand after optimization and before statepoint rewriting. Each group is
// bounded by memory operations other than its own stores. Reading all old
// values first and recording every new value also covers repeated destinations.
void lowerWriteBarrierRecords(Module &M) {
  configureWriteBarrierRecords(M);
  SmallVector<CallInst *, 32> Writes;
  SmallPtrSet<CallInst *, 32> ZeroOld;
  for (Function &F : M)
    if (F.hasGC() && F.getGC() == "goallc") {
      size_t Start = Writes.size();
      for (BasicBlock &BB : F)
        for (Instruction &I : BB)
          if (auto *CI = dyn_cast<CallInst>(&I))
            if (isWriteBarrierRecord(CI))
              Writes.push_back(CI);
      if (Writes.size() != Start)
        findZeroOldValues(F, ArrayRef(Writes).drop_front(Start), ZeroOld);
    }
  if (Writes.empty())
    return;
  LLVMContext &C = M.getContext();
  Type *Word = M.getDataLayout().getIntPtrType(C);
  auto Flag = M.getOrInsertGlobal("runtime.writeBarrier", Type::getInt32Ty(C));
  Function *Reserve =
      Intrinsic::getOrInsertDeclaration(&M, Intrinsic::go_gc_write_barrier);
  SmallPtrSet<CallInst *, 32> Done;
  // SimplifyCFG can merge calls with different constant omission proofs into
  // a call with a select/PHI argument. Discard those frontend omissions unless
  // constant; independent LLVM proofs below can still omit a pointer.
  auto flags = [](CallInst *CI) -> unsigned {
    auto *C = dyn_cast<ConstantInt>(CI->getArgOperand(2));
    return C ? C->getZExtValue() : 0;
  };
  auto needOld = [&](CallInst *CI) {
    return !(flags(CI) & 1) && !ZeroOld.contains(CI);
  };
  auto needNew = [&](CallInst *CI) {
    Value *V = CI->getArgOperand(0);
    return !(flags(CI) & 2) && !isa<ConstantPointerNull>(V) &&
           !isa<GlobalValue>(V->stripPointerCasts());
  };
  for (CallInst *First : Writes) {
    if (Done.contains(First))
      continue;
    SmallVector<CallInst *, 8> Group{First};
    unsigned Count = unsigned(needOld(First)) + unsigned(needNew(First));
    SmallVector<Instruction *, 8> Pending, Hoist;
    for (Instruction *I = First->getNextNode(); I; I = I->getNextNode()) {
      if (auto *CI = dyn_cast<CallInst>(I)) {
        if (!isWriteBarrierRecord(CI))
          break;
        unsigned N = unsigned(needOld(CI)) + unsigned(needNew(CI));
        if (Count + N > 8)
          break;
        Group.push_back(CI);
        Count += N;
        Hoist.append(Pending);
        Pending.clear();
      } else {
        auto *Store = dyn_cast<StoreInst>(I);
        CallInst *Last = Group.back();
        if (Store && Store->isSimple() &&
            Store->getPointerOperand() == Last->getArgOperand(1) &&
            Store->getValueOperand() == Last->getArgOperand(0))
          continue;
        if (I->isTerminator() || I->mayReadOrWriteMemory() ||
            !isSafeToSpeculativelyExecute(I))
          break;
        Pending.push_back(I);
      }
    }
    for (Instruction *I : Hoist)
      I->moveBefore(First->getIterator());
    IRBuilder<> B(First);
    B.SetCurrentDebugLocation(First->getDebugLoc());
    if (Count) {
      Value *Enabled =
          B.CreateICmpNE(B.CreateLoad(B.getInt32Ty(), Flag), B.getInt32(0));
      Instruction *Then = SplitBlockAndInsertIfThen(
          Enabled, First, false, MDBuilder(C).createBranchWeights(1, 2000));
      B.SetInsertPoint(Then);
      struct Entry {
        Value *Pointer;
        bool LoadOld;
      };
      SmallVector<Entry, 8> Entries;
      SmallPtrSet<Value *, 8> NewSeen, OldSeen;
      for (CallInst *CI : Group) {
        Value *New = CI->getArgOperand(0), *Dst = CI->getArgOperand(1);
        if (needNew(CI) && NewSeen.insert(New).second)
          Entries.push_back({New, false});
        if (needOld(CI) && OldSeen.insert(Dst).second)
          Entries.push_back({Dst, true});
      }
      Value *Buf =
          B.CreateCall(Reserve, {B.getInt32(Entries.size())}, "wb.buf");
      // Reserve before reading old pointers so those temporary values do not
      // stay live across the call. Fill each entry immediately; all target
      // stores remain after the complete, non-preemptible buffer fill.
      for (unsigned I = 0; I < Entries.size(); ++I) {
        Value *Slot = B.CreateGEP(Word, Buf, B.getInt32(I));
        const Entry &E = Entries[I];
        Value *Pointer = E.LoadOld ? B.CreateLoad(Word, E.Pointer, "wb.old")
                                   : B.CreatePtrToInt(E.Pointer, Word);
        B.CreateStore(Pointer, Slot);
      }
    }
    for (CallInst *CI : Group) {
      Done.insert(CI);
      CI->eraseFromParent();
    }
  }
  M.getFunction(RecordName)->eraseFromParent();

  // Inferred call-site memory summaries predate the added runtime accesses.
  for (Function &F : M) {
    if (!F.isDeclaration()) {
      F.setMemoryEffects(MemoryEffects::unknown());
      F.removeFnAttr(Attribute::NoSync);
      F.removeFnAttr(Attribute::NoFree);
      F.removeFnAttr(Attribute::Speculatable);
      for (Argument &A : F.args())
        A.removeAttr(Attribute::WriteOnly);
    }
    for (BasicBlock &BB : F)
      for (Instruction &I : BB)
        if (auto *CB = dyn_cast<CallBase>(&I))
          if (!isa<IntrinsicInst>(CB)) {
            CB->removeFnAttr(Attribute::Memory);
            CB->removeFnAttr(Attribute::NoSync);
            CB->removeFnAttr(Attribute::NoFree);
          }
  }
}

} // namespace llvm::goallc
