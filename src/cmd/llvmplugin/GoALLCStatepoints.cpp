// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCStatepoints.h"
#include "llvm/ADT/MapVector.h"
#include "llvm/ADT/STLExtras.h"
#include "llvm/ADT/SetVector.h"
#include "llvm/ADT/SmallPtrSet.h"
#include "llvm/ADT/SmallSet.h"
#include "llvm/ADT/StringRef.h"
#include "llvm/IR/CallingConv.h"
#include "llvm/IR/Constants.h"
#include "llvm/IR/Dominators.h"
#include "llvm/IR/GCStrategy.h"
#include "llvm/IR/IRBuilder.h"
#include "llvm/IR/InstIterator.h"
#include "llvm/IR/Instructions.h"
#include "llvm/IR/IntrinsicInst.h"
#include "llvm/IR/Module.h"
#include "llvm/IR/Statepoint.h"
#include "llvm/IR/Verifier.h"
#include "llvm/Support/Error.h"
#include "llvm/Transforms/Utils/PromoteMemToReg.h"

#include <optional>
#include <string>

using namespace llvm;

namespace {

constexpr StringLiteral GoALLCGCName = "goallc";
constexpr StringLiteral GCLeafAttr = "gc-leaf-function";
constexpr StringLiteral GoResultsTupleAttr = "go_results_tuple";

// This strategy exists for statepoint verification and lowering. GoALLC owns
// statepoint insertion, so UseRS4GC deliberately remains false.
class GoALLCGCStrategy final : public GCStrategy {
public:
  GoALLCGCStrategy() {
    UseStatepoints = true;
    UseRS4GC = false;
    NeededSafePoints = false;
    // The GoALLC metadata printer consumes the Machine StackMaps records at
    // AsmPrinter finalization and hands their raw locations to the GoObj
    // writer. It does not use the legacy gc.root metadata model.
    UsesMetadata = true;
  }

  // Opaque pointers do not carry enough information to classify Go pointers.
  // The GoALLC pass performs value-level classification and remains
  // conservative for the first implementation.
  std::optional<bool> isGCManagedPointer(const Type *) const override {
    return std::nullopt;
  }
};

static GCRegistry::Add<GoALLCGCStrategy>
    GoALLCGCRegistration(GoALLCGCName,
                         "GoALLC value-level statepoint strategy");

using ValueSet = SetVector<Value *>;

struct LivenessData {
  MapVector<BasicBlock *, ValueSet> Kill;
  MapVector<BasicBlock *, ValueSet> Gen;
  MapVector<BasicBlock *, ValueSet> LiveIn;
  MapVector<BasicBlock *, ValueSet> LiveOut;
};

struct SafepointRecord {
  CallInst *Call;
  uint64_t ID;
  ValueSet Live;
  CallInst *Statepoint = nullptr;
  SmallVector<CallInst *, 8> Relocates;
};

struct AggregateLeaf {
  SmallVector<unsigned, 4> Indices;
};

enum class LivenessKind {
  PointerAggregates,
  ScalarPointers,
};

bool isGoCallingConv(CallingConv::ID CC) {
  return CC == CallingConv::GoABIInternal || CC == CallingConv::GoABI0;
}

bool containsPointer(Type *Ty) {
  if (Ty->isPointerTy())
    return true;
  if (auto *ST = dyn_cast<StructType>(Ty))
    return llvm::any_of(ST->elements(), containsPointer);
  if (auto *AT = dyn_cast<ArrayType>(Ty))
    return containsPointer(AT->getElementType());
  if (auto *VT = dyn_cast<VectorType>(Ty))
    return containsPointer(VT->getElementType());
  return false;
}

bool isTrackedValue(const Value *V, LivenessKind Kind) {
  if (isa<Constant>(V))
    return false;
  Type *Ty = V->getType();
  switch (Kind) {
  case LivenessKind::PointerAggregates:
    return !Ty->isPointerTy() && containsPointer(Ty);
  case LivenessKind::ScalarPointers:
    return Ty->isPointerTy();
  }
  llvm_unreachable("unknown GoALLC liveness kind");
}

void addLiveValue(Value *V, ValueSet &Live, LivenessKind Kind) {
  if (isTrackedValue(V, Kind))
    Live.insert(V);
}

bool isLeafCall(const CallBase &Call) {
  if (Call.hasFnAttr(GCLeafAttr))
    return true;
  if (const Function *Callee = Call.getCalledFunction())
    return Callee->isIntrinsic() || Callee->hasFnAttribute(GCLeafAttr);
  return false;
}

uint64_t stableStatepointID(StringRef FunctionName, uint64_t CallOrdinal) {
  uint64_t Hash = 14695981039346656037ULL;
  auto Mix = [&](uint8_t Byte) {
    Hash ^= Byte;
    Hash *= 1099511628211ULL;
  };
  for (char Byte : FunctionName)
    Mix(static_cast<uint8_t>(Byte));
  Mix(0);
  for (unsigned I = 0; I != sizeof(CallOrdinal); ++I)
    Mix(static_cast<uint8_t>(CallOrdinal >> (I * 8)));
  return Hash;
}

void scanBackward(BasicBlock::reverse_iterator Begin,
                  BasicBlock::reverse_iterator End, ValueSet &Live,
                  LivenessKind Kind) {
  for (Instruction &I : make_range(Begin, End)) {
    Live.remove(&I);
    if (isa<PHINode>(I))
      continue;
    for (Value *Operand : I.operands())
      addLiveValue(Operand, Live, Kind);
  }
}

void seedPhiUses(BasicBlock &BB, ValueSet &LiveOut, LivenessKind Kind) {
  for (BasicBlock *Succ : successors(&BB)) {
    for (Instruction &I : *Succ) {
      auto *Phi = dyn_cast<PHINode>(&I);
      if (!Phi)
        break;
      Value *Incoming = Phi->getIncomingValueForBlock(&BB);
      addLiveValue(Incoming, LiveOut, Kind);
    }
  }
}

LivenessData computeLiveness(Function &F, LivenessKind Kind) {
  LivenessData Data;
  SmallSetVector<BasicBlock *, 32> Worklist;

  for (BasicBlock &BB : F) {
    ValueSet &Kill = Data.Kill[&BB];
    for (Instruction &I : BB)
      if (isTrackedValue(&I, Kind))
        Kill.insert(&I);

    ValueSet &Gen = Data.Gen[&BB];
    scanBackward(BB.rbegin(), BB.rend(), Gen, Kind);

    ValueSet &Out = Data.LiveOut[&BB];
    seedPhiUses(BB, Out, Kind);

    ValueSet &In = Data.LiveIn[&BB];
    In = Gen;
    In.set_union(Out);
    In.set_subtract(Kill);
    if (!In.empty())
      Worklist.insert_range(predecessors(&BB));
  }

  while (!Worklist.empty()) {
    BasicBlock *BB = Worklist.pop_back_val();
    ValueSet &Out = Data.LiveOut[BB];
    size_t OldSize = Out.size();
    for (BasicBlock *Succ : successors(BB))
      Out.set_union(Data.LiveIn[Succ]);
    if (Out.size() == OldSize)
      continue;

    ValueSet NewIn = Out;
    NewIn.set_union(Data.Gen[BB]);
    NewIn.set_subtract(Data.Kill[BB]);
    if (NewIn.size() != Data.LiveIn[BB].size()) {
      Data.LiveIn[BB] = std::move(NewIn);
      Worklist.insert_range(predecessors(BB));
    }
  }
  return Data;
}

ValueSet liveAtCall(CallInst &Call, LivenessData &Data, LivenessKind Kind) {
  ValueSet Live = Data.LiveOut[Call.getParent()];
  // Go scans the callee's incoming arguments through
  // FUNCDATA_ArgsPointerMaps. The caller's statepoint therefore contains only
  // values live after the call, not values whose sole use is the call itself.
  scanBackward(Call.getParent()->rbegin(), Call.getIterator().getReverse(),
               Live, Kind);
  Live.remove(&Call);
  return Live;
}

Error enumerateAggregateLeaves(Type *Ty, SmallVectorImpl<unsigned> &Path,
                               SmallVectorImpl<AggregateLeaf> &Leaves) {
  if (auto *ST = dyn_cast<StructType>(Ty)) {
    if (ST->isOpaque())
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints cannot scalarize an opaque struct");
    for (auto [Index, ElementTy] : llvm::enumerate(ST->elements())) {
      Path.push_back(Index);
      if (Error Err = enumerateAggregateLeaves(ElementTy, Path, Leaves))
        return Err;
      Path.pop_back();
    }
    return Error::success();
  }
  if (auto *AT = dyn_cast<ArrayType>(Ty)) {
    for (uint64_t Index = 0; Index != AT->getNumElements(); ++Index) {
      Path.push_back(Index);
      if (Error Err =
              enumerateAggregateLeaves(AT->getElementType(), Path, Leaves))
        return Err;
      Path.pop_back();
    }
    return Error::success();
  }
  if (Ty->isVectorTy())
    return createStringError(
        std::errc::not_supported,
        "GoALLC statepoints do not support vectors inside live pointer "
        "aggregates");
  if (!Ty->isIntegerTy() && !Ty->isFloatingPointTy() && !Ty->isPointerTy())
    return createStringError(
        std::errc::not_supported,
        "GoALLC statepoints require scalar first-class aggregate leaves");
  Leaves.push_back({SmallVector<unsigned, 4>(ArrayRef<unsigned>(Path))});
  return Error::success();
}

std::string leafName(Value &Aggregate, ArrayRef<unsigned> Indices) {
  std::string Name =
      Aggregate.hasName() ? (Aggregate.getName() + ".leaf").str() : "agg.leaf";
  for (unsigned Index : Indices)
    Name += "." + std::to_string(Index);
  return Name;
}

Expected<SmallVector<Value *, 8>>
extractAggregateLeaves(Value &Aggregate, ArrayRef<AggregateLeaf> Leaves,
                       Function &F) {
  IRBuilder<> Builder(F.getContext());
  if (auto *Arg = dyn_cast<Argument>(&Aggregate)) {
    Builder.SetInsertPoint(&F.getEntryBlock(),
                           F.getEntryBlock().getFirstInsertionPt());
    (void)Arg;
  } else {
    auto *Definition = cast<Instruction>(&Aggregate);
    if (auto *Call = dyn_cast<CallInst>(Definition);
        Call && Call->isMustTailCall())
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints cannot scalarize a musttail aggregate result");
    if (isa<PHINode>(Definition))
      Builder.SetInsertPoint(Definition->getParent(),
                             Definition->getParent()->getFirstNonPHIIt());
    else if (Instruction *Next = Definition->getNextNode())
      Builder.SetInsertPoint(Next);
    else
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints cannot extract leaves after an aggregate "
          "terminator");
    Builder.SetCurrentDebugLocation(Definition->getDebugLoc());
  }

  SmallVector<Value *, 8> Values;
  Values.reserve(Leaves.size());
  for (const AggregateLeaf &Leaf : Leaves)
    Values.push_back(Builder.CreateExtractValue(
        &Aggregate, Leaf.Indices, leafName(Aggregate, Leaf.Indices)));
  return Values;
}

Instruction *aggregateUseInsertionPoint(Use &U) {
  auto *User = dyn_cast<Instruction>(U.getUser());
  if (!User)
    return nullptr;
  if (auto *Phi = dyn_cast<PHINode>(User))
    return Phi->getIncomingBlock(U)->getTerminator();
  return User;
}

Error scalarizeLivePointerAggregates(Function &F) {
  LivenessData AggregateLiveness =
      computeLiveness(F, LivenessKind::PointerAggregates);
  ValueSet Candidates;

  for (Instruction &I : instructions(F)) {
    auto *Call = dyn_cast<CallBase>(&I);
    if (!Call || isa<GCStatepointInst>(Call) || isLeafCall(*Call))
      continue;
    auto *OrdinaryCall = dyn_cast<CallInst>(Call);
    if (!OrdinaryCall)
      continue;
    Candidates.insert_range(liveAtCall(*OrdinaryCall, AggregateLiveness,
                                       LivenessKind::PointerAggregates));
  }

  for (Value *Candidate : Candidates) {
    SmallVector<AggregateLeaf, 8> Leaves;
    SmallVector<unsigned, 4> Path;
    if (Error Err =
            enumerateAggregateLeaves(Candidate->getType(), Path, Leaves))
      return std::move(Err);

    SmallVector<Use *, 16> OriginalUses;
    for (Use &U : Candidate->uses())
      OriginalUses.push_back(&U);

    Expected<SmallVector<Value *, 8>> LeafValuesOrErr =
        extractAggregateLeaves(*Candidate, Leaves, F);
    if (!LeafValuesOrErr)
      return LeafValuesOrErr.takeError();
    SmallVector<Value *, 8> LeafValues = std::move(*LeafValuesOrErr);

    for (Use *U : OriginalUses) {
      auto *Extract = dyn_cast<ExtractValueInst>(U->getUser());
      if (Extract) {
        auto Match = llvm::find_if(Leaves, [&](const AggregateLeaf &Leaf) {
          return ArrayRef<unsigned>(Leaf.Indices) == Extract->getIndices();
        });
        if (Match != Leaves.end()) {
          size_t Index = std::distance(Leaves.begin(), Match);
          Extract->replaceAllUsesWith(LeafValues[Index]);
          Extract->eraseFromParent();
          continue;
        }
      }

      Instruction *InsertBefore = aggregateUseInsertionPoint(*U);
      if (!InsertBefore)
        return createStringError(
            std::errc::not_supported,
            "GoALLC statepoints cannot rebuild a non-instruction aggregate "
            "use");
      IRBuilder<> Builder(InsertBefore);
      Builder.SetCurrentDebugLocation(InsertBefore->getDebugLoc());
      Value *Rebuilt = PoisonValue::get(Candidate->getType());
      for (auto [Leaf, LeafValue] : llvm::zip(Leaves, LeafValues)) {
        Rebuilt = Builder.CreateInsertValue(
            Rebuilt, LeafValue, Leaf.Indices,
            Candidate->hasName() ? Candidate->getName() + ".rebuilt" : "");
      }
      U->set(Rebuilt);
    }
    for (Value *LeafValue : LeafValues)
      if (auto *LeafInst = dyn_cast<Instruction>(LeafValue);
          LeafInst && LeafInst->use_empty())
        LeafInst->eraseFromParent();
  }
  return Error::success();
}

Error validateMemoryObjects(Function &F) {
  bool HasSafepoint = llvm::any_of(instructions(F), [](Instruction &I) {
    auto *Call = dyn_cast<CallBase>(&I);
    return Call && !isa<GCStatepointInst>(Call) && !isLeafCall(*Call);
  });
  if (!HasSafepoint)
    return Error::success();
  for (Instruction &I : instructions(F)) {
    auto *Alloca = dyn_cast<AllocaInst>(&I);
    if (Alloca && containsPointer(Alloca->getAllocatedType()))
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints do not support pointer-containing memory "
          "objects; SSA aggregate scalarization cannot describe their "
          "internal pointer slots");
  }
  return Error::success();
}

Error validateSafepoint(const SafepointRecord &Record) {
  const CallInst &Call = *Record.Call;
  if (Call.isInlineAsm())
    return createStringError(
        std::errc::not_supported,
        "GoALLC statepoints do not support non-leaf inline assembly");
  if (Call.isMustTailCall())
    return createStringError(std::errc::not_supported,
                             "GoALLC statepoints do not support musttail");
  if (Call.getNumOperandBundles() != 0)
    return createStringError(
        std::errc::not_supported,
        "GoALLC statepoints do not yet support call operand bundles");
  for (unsigned I = 0; I != Call.arg_size(); ++I) {
    for (Attribute Attr : Call.getAttributes().getParamAttrs(I)) {
      if (!Attr.hasAttribute(Attribute::Nest))
        return createStringError(
            std::errc::not_supported,
            "GoALLC statepoints only support the nest call parameter "
            "attribute");
    }
  }
  for (Value *V : Record.Live) {
    if (!V->getType()->isPointerTy())
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints do not yet support live pointer aggregates");
  }
  return Error::success();
}

Error rewriteCall(SafepointRecord &Record) {
  CallInst *Call = Record.Call;

  SmallVector<Value *, 8> CallArgs(Call->args());
  SmallVector<Value *, 8> GCLive(Record.Live.begin(), Record.Live.end());
  FunctionCallee Callee(Call->getFunctionType(), Call->getCalledOperand());

  IRBuilder<> Builder(Call);
  Builder.SetCurrentDebugLocation(Call->getDebugLoc());
  Record.Statepoint = Builder.CreateGCStatepointCall(
      Record.ID, 0, Callee, CallArgs, std::nullopt, GCLive, "statepoint_token");
  Record.Statepoint->setCallingConv(Call->getCallingConv());
  if (Call->hasFnAttr(GoResultsTupleAttr))
    Record.Statepoint->addFnAttr(
        Attribute::get(Call->getContext(), GoResultsTupleAttr));
  for (unsigned I = 0; I != Call->arg_size(); ++I) {
    for (Attribute Attr : Call->getAttributes().getParamAttrs(I))
      Record.Statepoint->addParamAttr(GCStatepointInst::CallArgsBeginPos + I,
                                      Attr);
  }

  Instruction *InsertBefore = Call->getNextNode();
  Builder.SetInsertPoint(InsertBefore);
  Builder.SetCurrentDebugLocation(Call->getDebugLoc());

  CallInst *Result = nullptr;
  if (!Call->getType()->isVoidTy() && !Call->use_empty()) {
    Result = Builder.CreateGCResult(Record.Statepoint, Call->getType());
    Result->setAttributes(
        AttributeList::get(Call->getContext(), AttributeList::ReturnIndex,
                           Call->getAttributes().getRetAttrs()));
  }

  for (auto [Index, V] : llvm::enumerate(GCLive)) {
    CallInst *Relocate = Builder.CreateGCRelocate(
        Record.Statepoint, Index, Index, V->getType(),
        V->hasName() ? V->getName() + ".relocated" : "");
    Relocate->setCallingConv(CallingConv::Cold);
    Record.Relocates.push_back(Relocate);
  }

  if (Result)
    Call->replaceAllUsesWith(Result);
  Call->eraseFromParent();
  Record.Call = nullptr;
  return Error::success();
}

void repairRelocationSSA(Function &F, DominatorTree &DT,
                         ArrayRef<SafepointRecord> Records) {
  // Re-read gc-live after every ordinary call has been replaced. A
  // pointer-valued call in an earlier record may now be a gc.result operand of
  // a later statepoint, so the pre-rewrite liveness records can contain erased
  // instructions.
  ValueSet Live;
  for (const SafepointRecord &Record : Records)
    Live.insert_range(cast<GCStatepointInst>(Record.Statepoint)->gc_live());
  if (Live.empty())
    return;

  const DataLayout &DL = F.getDataLayout();
  MapVector<Value *, AllocaInst *> Slots;
  SmallVector<AllocaInst *, 16> PromotableAllocas;
  PromotableAllocas.reserve(Live.size());
  for (Value *V : Live) {
    StringRef Name = V->hasName() ? V->getName() : "pointer";
    auto *Slot = new AllocaInst(V->getType(), DL.getAllocaAddrSpace(),
                                (Name + ".relocated.merge").str(),
                                F.getEntryBlock().getFirstNonPHIIt());
    Slots[V] = Slot;
    PromotableAllocas.push_back(Slot);
  }

  // A relocate is a new reaching definition of its original gc-live value.
  // Insert these stores before rewriting the statepoint operands themselves:
  // getDerivedPtr() still identifies the alloca which owns each relocate.
  for (const SafepointRecord &Record : Records) {
    for (CallInst *RelocateCall : Record.Relocates) {
      auto *Relocate = cast<GCRelocateInst>(RelocateCall);
      Value *Original = Relocate->getDerivedPtr();
      auto Slot = Slots.find(Original);
      assert(Slot != Slots.end() && "relocate is missing its gc-live value");
      new StoreInst(RelocateCall, Slot->second,
                    std::next(RelocateCall->getIterator()));
    }
  }

  // Express every old use as a load from the pointer's temporary slot, then
  // seed that slot immediately after the original definition. PromoteMemToReg
  // removes all of this memory traffic and constructs the required SSA PHIs
  // for arbitrary CFGs, including loop backedges and irreducible regions.
  for (auto [Original, Slot] : Slots) {
    SmallVector<Instruction *, 16> Users;
    SmallPtrSet<Instruction *, 16> Seen;
    for (User *U : Original->users())
      if (auto *I = dyn_cast<Instruction>(U); I && Seen.insert(I).second)
        Users.push_back(I);

    StringRef Name = Original->hasName() ? Original->getName() : "pointer";
    std::string LoadName = (Name + ".relocated.current").str();
    for (Instruction *User : Users) {
      if (auto *Phi = dyn_cast<PHINode>(User)) {
        for (unsigned I = 0; I != Phi->getNumIncomingValues(); ++I) {
          if (Phi->getIncomingValue(I) != Original)
            continue;
          auto *Load = new LoadInst(
              Original->getType(), Slot, LoadName,
              Phi->getIncomingBlock(I)->getTerminator()->getIterator());
          Phi->setIncomingValue(I, Load);
        }
        continue;
      }
      auto *Load = new LoadInst(Original->getType(), Slot, LoadName,
                                User->getIterator());
      User->replaceUsesOfWith(Original, Load);
    }

    auto *Store = new StoreInst(Original, Slot, false,
                                DL.getABITypeAlign(Original->getType()));
    if (auto *Definition = dyn_cast<Instruction>(Original)) {
      if (isa<PHINode>(Definition))
        Store->insertBefore(Definition->getParent()->getFirstNonPHIIt());
      else {
        assert(!Definition->isTerminator() &&
               "GoALLC does not support value-producing terminators");
        Store->insertAfter(Definition->getIterator());
      }
    } else {
      assert(isa<Argument>(Original) && "expected local pointer definition");
      Store->insertAfter(Slot->getIterator());
    }
  }

  PromoteMemToReg(PromotableAllocas, DT);
}

Error rewriteFunction(Function &F) {
  if (F.hasFnAttribute(GCLeafAttr)) {
    for (Instruction &I : instructions(F)) {
      auto *Call = dyn_cast<CallBase>(&I);
      if (!Call)
        continue;
      if (isa<GCStatepointInst>(Call) || !isLeafCall(*Call))
        return createStringError(
            std::errc::invalid_argument,
            "GoALLC gc-leaf-function contains a non-leaf call");
    }
    return Error::success();
  }

  if (Error Err = validateMemoryObjects(F))
    return Err;
  if (Error Err = scalarizeLivePointerAggregates(F))
    return Err;

  DominatorTree DT(F);
  LivenessData Data = computeLiveness(F, LivenessKind::ScalarPointers);
  SmallVector<SafepointRecord, 8> Records;
  uint64_t CallOrdinal = 0;

  for (Instruction &I : instructions(F)) {
    auto *Call = dyn_cast<CallBase>(&I);
    if (!Call || isa<GCStatepointInst>(Call) || isLeafCall(*Call))
      continue;
    auto *OrdinaryCall = dyn_cast<CallInst>(Call);
    if (!OrdinaryCall)
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints do not yet support invoke or callbr");
    Records.push_back(
        {OrdinaryCall, stableStatepointID(F.getName(), CallOrdinal++),
         liveAtCall(*OrdinaryCall, Data, LivenessKind::ScalarPointers)});
  }
  for (const SafepointRecord &Record : Records)
    if (Error Err = validateSafepoint(Record))
      return Err;
  for (SafepointRecord &Record : llvm::reverse(Records))
    if (Error Err = rewriteCall(Record))
      return Err;
  repairRelocationSSA(F, DT, Records);
  return Error::success();
}

} // namespace

Error goallc::rewriteStatepoints(Module &M, TargetMachine &) {
  for (Function &F : M) {
    if (F.isDeclaration() || !isGoCallingConv(F.getCallingConv()) ||
        !F.hasGC() || F.getGC() != GoALLCGCName)
      continue;
    if (Error Err = rewriteFunction(F))
      return Err;
  }
  if (verifyModule(M, &errs()))
    return createStringError(std::errc::invalid_argument,
                             "GoALLC statepoint rewrite produced invalid IR");
  return Error::success();
}
