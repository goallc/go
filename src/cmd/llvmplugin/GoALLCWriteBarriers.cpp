// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCWriteBarriers.h"
#include "llvm/ADT/SmallPtrSet.h"
#include "llvm/ADT/SmallVector.h"
#include "llvm/Analysis/ValueTracking.h"
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
  for (Function &F : M)
    if (F.hasGC() && F.getGC() == "goallc")
      for (BasicBlock &BB : F)
        for (Instruction &I : BB)
          if (auto *CI = dyn_cast<CallInst>(&I))
            if (isWriteBarrierRecord(CI))
              Writes.push_back(CI);
  if (Writes.empty())
    return;
  LLVMContext &C = M.getContext();
  Type *Word = M.getDataLayout().getIntPtrType(C);
  auto Flag = M.getOrInsertGlobal("runtime.writeBarrier", Type::getInt32Ty(C));
  Function *Reserve =
      Intrinsic::getOrInsertDeclaration(&M, Intrinsic::go_gc_write_barrier);
  SmallPtrSet<CallInst *, 32> Done;
  // SimplifyCFG can merge calls with different constant omission proofs into
  // a call with a select/PHI argument. Without a constant proof, conservatively
  // record both pointers. Losing an omission only adds redundant GC work.
  auto flags = [](CallInst *CI) -> unsigned {
    auto *C = dyn_cast<ConstantInt>(CI->getArgOperand(2));
    return C ? C->getZExtValue() : 0;
  };
  auto needOld = [&](CallInst *CI) { return !(flags(CI) & 1); };
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
      SmallVector<Value *, 8> Entries;
      SmallPtrSet<Value *, 8> NewSeen, OldSeen;
      for (CallInst *CI : Group) {
        Value *New = CI->getArgOperand(0), *Dst = CI->getArgOperand(1);
        if (needNew(CI) && NewSeen.insert(New).second)
          Entries.push_back(B.CreatePtrToInt(New, Word));
        if (needOld(CI) && OldSeen.insert(Dst).second)
          Entries.push_back(B.CreateLoad(Word, Dst, "wb.old"));
      }
      Value *Buf =
          B.CreateCall(Reserve, {B.getInt32(Entries.size())}, "wb.buf");
      for (unsigned I = 0; I < Entries.size(); ++I)
        B.CreateStore(Entries[I], B.CreateGEP(Word, Buf, B.getInt32(I)));
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
