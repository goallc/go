// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCPreCodeGen.h"
#include "GoALLCStatepoints.h"
#include "llvm/ADT/SmallBitVector.h"
#include "llvm/ADT/SmallPtrSet.h"
#include "llvm/ADT/SmallVector.h"
#include "llvm/Analysis/ValueTracking.h"
#include "llvm/CodeGen/GoCallingConv.h"
#include "llvm/IR/CFG.h"
#include "llvm/IR/Constants.h"
#include "llvm/IR/IRBuilder.h"
#include "llvm/IR/Instructions.h"
#include "llvm/IR/IntrinsicInst.h"
#include "llvm/IR/Module.h"
#include "llvm/Target/TargetMachine.h"

#include <algorithm>

using namespace llvm;

namespace {

// This is a preparation for the existing single-call carrier proof, not a
// relaxation of it. Keep the object typed and retain pointer-valued stores;
// statepoint rewriting must see their final placement before computing roots.
bool prepareByValCallArgument(AllocaInst &AI) {
  const DataLayout &DL = AI.getModule()->getDataLayout();
  auto Size = AI.getAllocationSize(DL);
  auto *Count = dyn_cast<ConstantInt>(AI.getArraySize());
  if (!AI.isStaticAlloca() || !Count || !Count->isOne() ||
      AI.getAddressSpace() != 0 || !Size || Size->isScalable() ||
      Size->getFixedValue() == 0 || Size->getFixedValue() > 256)
    return false;
  // These frame homes have Go unwind/defer semantics beyond ordinary uses.
  if (AI.getMetadata("goallc.defer_result") ||
      AI.getMetadata("goallc.open_defer_bits") ||
      AI.getMetadata("goallc.open_defer_slots"))
    return false;
  const uint64_t Bytes = Size->getFixedValue();

  SmallVector<Value *, 16> Worklist{&AI};
  SmallPtrSet<Value *, 16> Seen;
  SmallVector<Instruction *, 16> Addresses;
  SmallPtrSet<StoreInst *, 16> Stores;
  IntrinsicInst *Lifetime = nullptr;
  MemSetInst *Zero = nullptr;
  CallInst *Call = nullptr;
  for (unsigned I = 0; I != Worklist.size(); ++I) {
    Value *V = Worklist[I];
    if (!Seen.insert(V).second)
      continue;
    // Bound the work as well as the byte size for unusually fragmented IR.
    if (Seen.size() > 128)
      return false;
    for (Use &U : V->uses()) {
      auto *Inst = dyn_cast<Instruction>(U.getUser());
      if (!Inst)
        return false;
      if (isa<GetElementPtrInst, BitCastInst>(Inst)) {
        int64_t Offset = 0;
        if (U.getOperandNo() != 0 ||
            GetPointerBaseWithConstantOffset(Inst, Offset, DL) != &AI ||
            Offset < 0 || uint64_t(Offset) > Bytes)
          return false;
        Addresses.push_back(Inst);
        Worklist.push_back(Inst);
        continue;
      }
      if (auto *SI = dyn_cast<StoreInst>(Inst)) {
        Type *Ty = SI->getValueOperand()->getType();
        TypeSize StoreSize = DL.getTypeStoreSize(Ty);
        int64_t Offset = 0;
        if (U.getOperandNo() != StoreInst::getPointerOperandIndex() ||
            !SI->isSimple() || Ty->isAggregateType() ||
            StoreSize.isScalable() ||
            DL.getTypeSizeInBits(Ty) != StoreSize * 8 ||
            GetPointerBaseWithConstantOffset(V, Offset, DL) != &AI ||
            Offset < 0 || uint64_t(Offset) > Bytes ||
            StoreSize.getFixedValue() > Bytes - uint64_t(Offset))
          return false;
        Stores.insert(SI);
        if (Stores.size() > 128)
          return false;
        continue;
      }
      if (auto *MI = dyn_cast<MemSetInst>(Inst)) {
        auto *Length = dyn_cast<ConstantInt>(MI->getLength());
        auto *Byte = dyn_cast<ConstantInt>(MI->getValue());
        if (Zero || MI->isVolatile() || V != &AI || !Length ||
            Length->getValue() != Bytes || !Byte || !Byte->isZero())
          return false;
        Zero = MI;
        continue;
      }
      if (auto *II = dyn_cast<IntrinsicInst>(Inst)) {
        if (Lifetime || II->getIntrinsicID() != Intrinsic::lifetime_start ||
            V != &AI)
          return false;
        Lifetime = II;
        continue;
      }
      if (auto *CI = dyn_cast<CallInst>(Inst)) {
        if (Call || CI->isMustTailCall() || !CI->isArgOperand(&U) ||
            !goabi::isGoCallingConv(CI->getCallingConv()) || V != &AI)
          return false;
        unsigned Arg = CI->getArgOperandNo(&U);
        if (!CI->paramHasAttr(Arg, Attribute::ByVal) ||
            CI->getParamByValType(Arg) != AI.getAllocatedType() ||
            !CI->getParamAlign(Arg) || *CI->getParamAlign(Arg) != AI.getAlign())
          return false;
        Call = CI;
        continue;
      }
      // No loads, escaping addresses, goret uses, or pre-existing statepoints.
      return false;
    }
  }
  if (!Zero || !Call || Stores.empty())
    return false;

  // A chain of unique predecessors makes the initialization dominate the
  // call, including a nil-check edge. There can be no merge or loop between
  // them. Other successors may exit, but cannot observe this private object.
  SmallVector<BasicBlock *, 8> Blocks;
  BasicBlock *BB = Call->getParent();
  while (BB != Zero->getParent()) {
    if (!BB || Blocks.size() == 8 || llvm::is_contained(Blocks, BB))
      return false;
    Blocks.push_back(BB);
    BB = BB->getSinglePredecessor();
  }
  Blocks.push_back(BB);
  std::reverse(Blocks.begin(), Blocks.end());

  SmallVector<StoreInst *, 16> OrderedStores;
  bool InRange = false;
  bool ReachedCall = false;
  for (BasicBlock *Block : Blocks) {
    for (Instruction &I : *Block) {
      if (&I == Zero) {
        InRange = true;
        continue;
      }
      if (!InRange)
        continue;
      if (&I == Call) {
        ReachedCall = true;
        break;
      }
      if (auto *SI = dyn_cast<StoreInst>(&I); Stores.contains(SI))
        OrderedStores.push_back(SI);
      // Do not move a pointer snapshot across a safepoint or another call's
      // argument evaluation. Even a GC-leaf call can observe or reuse stack
      // arguments, and may have side effects that constrain evaluation.
      if (isa<CallBase>(I) && &I != Lifetime)
        return false;
    }
  }
  if (!ReachedCall || OrderedStores.size() != Stores.size())
    return false;
  if (Lifetime && (Lifetime->getParent() != Zero->getParent() ||
                   !Lifetime->comesBefore(Zero)))
    return false;

  // Preserve typed final stores. A fully overwritten store is dead, but
  // partially overwritten values require byte extraction and are left alone.
  SmallBitVector Covered(Bytes);
  SmallVector<StoreInst *, 16> FinalStores;
  for (StoreInst *SI : llvm::reverse(OrderedStores)) {
    int64_t Offset = 0;
    GetPointerBaseWithConstantOffset(SI->getPointerOperand(), Offset, DL);
    unsigned End =
        Offset +
        DL.getTypeStoreSize(SI->getValueOperand()->getType()).getFixedValue();
    bool Any = false, All = true;
    for (unsigned J = Offset; J != End; ++J) {
      Any |= Covered[J];
      All &= Covered[J];
    }
    if (All)
      continue;
    if (Any)
      return false;
    Covered.set(Offset, End);
    FinalStores.push_back(SI);
  }

  IRBuilder<> B(Call);
  B.SetCurrentDebugLocation(Zero->getDebugLoc());
  if (Lifetime)
    Lifetime->moveBefore(Call->getIterator());
  for (unsigned Begin = 0; Begin != Bytes;) {
    if (Covered[Begin]) {
      ++Begin;
      continue;
    }
    unsigned End = Begin + 1;
    while (End != Bytes && !Covered[End])
      ++End;
    Type *ZeroTy = ArrayType::get(B.getInt8Ty(), End - Begin);
    Value *Address = B.CreateConstGEP1_64(B.getInt8Ty(), &AI, Begin);
    B.CreateAlignedStore(Constant::getNullValue(ZeroTy), Address,
                         commonAlignment(AI.getAlign(), Begin));
    Begin = End;
  }
  for (StoreInst *SI : llvm::reverse(FinalStores)) {
    int64_t Offset = 0;
    GetPointerBaseWithConstantOffset(SI->getPointerOperand(), Offset, DL);
    B.SetCurrentDebugLocation(SI->getDebugLoc());
    Value *Address = B.CreateConstGEP1_64(B.getInt8Ty(), &AI, Offset);
    B.CreateAlignedStore(SI->getValueOperand(), Address, SI->getAlign());
  }
  for (StoreInst *SI : OrderedStores)
    SI->eraseFromParent();
  Zero->eraseFromParent();
  // Address users have all been replaced. Delete from leaves to roots so the
  // existing single-call proof sees no stale cross-block GEPs.
  for (Instruction *I : llvm::reverse(Addresses))
    if (I->use_empty())
      I->eraseFromParent();
  return true;
}

} // namespace

void goallc::prepareByValCallArguments(Function &F) {
  SmallVector<AllocaInst *, 16> Allocas;
  for (Instruction &I : F.getEntryBlock())
    if (auto *AI = dyn_cast<AllocaInst>(&I))
      Allocas.push_back(AI);
  for (AllocaInst *AI : Allocas)
    prepareByValCallArgument(*AI);
}

Error goallc::runPreCodeGenPipeline(Module &M, TargetMachine &TM) {
  return rewriteStatepoints(M, TM);
}
