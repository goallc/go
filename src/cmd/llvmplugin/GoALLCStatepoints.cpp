// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCStatepoints.h"
#include "llvm/ADT/APInt.h"
#include "llvm/ADT/DenseMap.h"
#include "llvm/ADT/MapVector.h"
#include "llvm/ADT/STLExtras.h"
#include "llvm/ADT/SetVector.h"
#include "llvm/ADT/SmallBitVector.h"
#include "llvm/ADT/SmallPtrSet.h"
#include "llvm/ADT/SmallSet.h"
#include "llvm/ADT/StringRef.h"
#include "llvm/Analysis/CFG.h"
#include "llvm/Analysis/ConstantFolding.h"
#include "llvm/Analysis/LoopInfo.h"
#include "llvm/Analysis/Utils/Local.h"
#include "llvm/Analysis/ValueTracking.h"
#include "llvm/BinaryFormat/GoObj.h"
#include "llvm/CodeGen/Analysis.h"
#include "llvm/CodeGen/GoCallingConv.h"
#include "llvm/IR/BasicBlock.h"
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
#include "llvm/IR/ValueHandle.h"
#include "llvm/IR/Verifier.h"
#include "llvm/Support/Alignment.h"
#include "llvm/Support/CheckedArithmetic.h"
#include "llvm/Support/Error.h"
#include "llvm/Transforms/Utils/BasicBlockUtils.h"
#include "llvm/Transforms/Utils/Local.h"
#include "llvm/Transforms/Utils/PromoteMemToReg.h"

#include <limits>
#include <optional>
#include <string>

using namespace llvm;

namespace {

constexpr StringLiteral GoALLCGCName = "goallc";
constexpr StringLiteral GCLeafAttr = "gc-leaf-function";
constexpr StringLiteral GoResultsTupleAttr = "go_results_tuple";
constexpr StringLiteral GoDeferResultMD = "goallc.defer_result";
constexpr StringLiteral GoOpenDeferBitsMD = "goallc.open_defer_bits";
constexpr StringLiteral GoOpenDeferSlotsMD = "goallc.open_defer_slots";
constexpr StringLiteral GoNotInHeapAddressMD = "goallc.notinheap";
constexpr StringLiteral GoObjMarkerRelocMD = "goobj.marker_reloc";
constexpr StringLiteral StackColoringNoMergeMD = "llvm.stackcoloring.no_merge";

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
using DirectPointerLeafAliasGroups =
    MapVector<Value *, SmallVector<Value *, 4>>;

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
  ValueSet FixedFrameAddresses;
  ValueSet DerivedPointers;
  // Aggregate scalarization gives a defined pointer leaf its own SSA identity
  // so relocating that leaf does not rewrite unrelated uses of the pointer
  // originally inserted into the aggregate. When both identities are live at
  // one safepoint, they can nevertheless share that safepoint's root and
  // relocated definition.
  SmallVector<std::pair<Value *, Value *>, 4> CoalescedLiveRoots;
  // Resolve the shared relocate while original call results still exist;
  // eraseOriginalCalls can replace and erase those root Value identities.
  SmallVector<std::pair<WeakTrackingVH, CallInst *>, 4> CoalescedRelocates;
  CallInst *Statepoint = nullptr;
  CallInst *Result = nullptr;
  SmallVector<CallInst *, 8> Relocates;
};

struct AggregateLeaf {
  SmallVector<unsigned, 4> Indices;
};

struct PointerFrameLeaf {
  SmallVector<unsigned, 4> Indices;
  uint64_t Offset;
  PointerType *Type;
};

struct PointerFrameLayout {
  uint64_t ByteSize;
  uint64_t Alignment;
  uint64_t BitCount;
  SmallVector<uint64_t, 4> BitmapWords;
  SmallVector<PointerFrameLeaf, 8> Leaves;
};

struct PointerAllocaRecord {
  AllocaInst *Alloca;
  bool NeedsStackObject;
  bool DeferResult;
  bool OpenDeferSlot;
  PointerFrameLayout Layout;
  SmallVector<IntrinsicInst *, 4> LifetimeMarkers;
  SmallVector<Instruction *, 16> AddressUses;
  SmallVector<CallInst *, 4> GoRetDefs;
  SmallVector<CallInst *, 8> ActiveCalls;
  bool WholeLifetime = false;
  bool ActivityUnclear = false;
};

struct PointerFixedArgRecord {
  Argument *Base;
  bool IsGoRet;
  bool NeedsStackObject;
  bool DeferResult;
  PointerFrameLayout Layout;
  DenseMap<const Instruction *, SmallBitVector> ContentUses;
  DenseMap<const Instruction *, SmallBitVector> ContentDefs;
  SmallVector<CallInst *, 8> ActiveCalls;
  bool ActivityUnclear = false;
};

struct OpenDeferInfo {
  AllocaInst *Bits = nullptr;
  AllocaInst *Slots = nullptr;
  uint64_t SlotCount = 0;
};

// Pointer-bearing SSA values share one liveness and profitability analysis,
// but not one post-statepoint representation. Keeping the strategy explicit
// prevents aggregate carriers from becoming mutable whole-value SSA
// definitions: only scalar pointers use gc.relocate, while homes remain
// authoritative memory and fixed/derived addresses are rematerialized.
enum class StatepointPreservationStrategy {
  RelocateSSA,
  TrackInHome,
  Rematerialize,
};

struct StatepointPreservationPlan {
  Value *V;
  StatepointPreservationStrategy Strategy =
      StatepointPreservationStrategy::RelocateSSA;
  SmallVector<CallInst *, 4> LiveCalls;
  SmallVector<Loop *, 2> HomeLoops;
};

// Classify how a frame-derived address participates in IR. Address derivations
// and bookkeeping remain structurally tied to the frame object. Terminal
// memory uses can be rebuilt immediately at the access. First-class uses are
// either normalized to Base+Offset for one fixed object or left on the generic
// relocatable-pointer path when their provenance is ambiguous.
enum class FrameAddressUseKind {
  Derivation,
  TerminalMemory,
  LifetimeOrDebug,
  FakeUse,
  FirstClass,
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
    return AT->getNumElements() != 0 && containsPointer(AT->getElementType());
  if (auto *VT = dyn_cast<VectorType>(Ty))
    return containsPointer(VT->getElementType());
  return false;
}

bool isRelocatablePointerType(Type *Ty) {
  if (Ty->isPointerTy())
    return true;
  auto *VT = dyn_cast<FixedVectorType>(Ty);
  return VT && VT->getElementType()->isPointerTy();
}

bool isFixedFrameArgument(const Value *V) {
  const auto *Arg = dyn_cast<Argument>(V);
  return Arg && (Arg->hasByValAttr() || Arg->hasGoRetAttr());
}

bool isFixedFrameBase(const Value *V) {
  if (const auto *Alloca = dyn_cast<AllocaInst>(V))
    return Alloca->isStaticAlloca();
  return isFixedFrameArgument(V);
}

// Return the one fixed frame object denoted by Root when every path through its
// forwarding graph remains derived from that object. Offsets may differ: they
// are normalized to integer SSA separately, so only mixed object identities
// make the provenance ambiguous.
const Value *fixedFrameProvenanceBase(const Value *Root) {
  if (!Root->getType()->isPointerTy())
    return nullptr;

  struct IdentityQuery {
    const Value *V;
    SmallVector<unsigned, 4> Indices;
  };

  SmallVector<IdentityQuery, 8> Worklist;
  Worklist.push_back({Root, {}});
  SmallVector<IdentityQuery, 8> Visited;
  const Value *Base = nullptr;

  while (!Worklist.empty()) {
    IdentityQuery Query = Worklist.pop_back_val();
    if (llvm::any_of(Visited, [&](const IdentityQuery &Seen) {
          return Seen.V == Query.V && Seen.Indices == Query.Indices;
        }))
      continue;
    Visited.push_back(Query);

    auto Push = [&](const Value *V, ArrayRef<unsigned> Indices) {
      IdentityQuery Next{V, {}};
      Next.Indices.append(Indices.begin(), Indices.end());
      Worklist.push_back(std::move(Next));
    };

    const Value *V = Query.V;
    if (!Query.Indices.empty()) {
      // Carry an aggregate leaf path through SSA forwarding before asking
      // FindInsertedValue to resolve an insertvalue chain. This is the form
      // produced when aggregate scalarization extracts a pointer leaf from an
      // aggregate PHI/select that is live across a statepoint.
      if (const auto *Phi = dyn_cast<PHINode>(V)) {
        for (const Value *Incoming : Phi->incoming_values())
          Push(Incoming, Query.Indices);
        continue;
      }
      if (const auto *Select = dyn_cast<SelectInst>(V)) {
        Push(Select->getTrueValue(), Query.Indices);
        Push(Select->getFalseValue(), Query.Indices);
        continue;
      }
      if (const auto *Freeze = dyn_cast<FreezeInst>(V)) {
        Push(Freeze->getOperand(0), Query.Indices);
        continue;
      }
      if (const auto *Extract = dyn_cast<ExtractValueInst>(V)) {
        SmallVector<unsigned, 4> Combined(Extract->getIndices());
        Combined.append(Query.Indices.begin(), Query.Indices.end());
        Push(Extract->getAggregateOperand(), Combined);
        continue;
      }
      if (const auto *Insert = dyn_cast<InsertValueInst>(V)) {
        ArrayRef<unsigned> InsertIndices = Insert->getIndices();
        ArrayRef<unsigned> QueryIndices = Query.Indices;
        if (QueryIndices.size() >= InsertIndices.size() &&
            llvm::equal(InsertIndices,
                        QueryIndices.take_front(InsertIndices.size()))) {
          // The requested leaf belongs to the inserted value. Continue with
          // the suffix relative to that value, including an empty suffix when
          // the inserted value is itself the pointer leaf.
          Push(Insert->getInsertedValueOperand(),
               QueryIndices.drop_front(InsertIndices.size()));
        } else {
          // This insertion updates a disjoint aggregate path. The requested
          // leaf still comes from the input aggregate.
          Push(Insert->getAggregateOperand(), QueryIndices);
        }
        continue;
      }
      if (Value *Inserted =
              FindInsertedValue(const_cast<Value *>(V), Query.Indices)) {
        Push(Inserted, {});
        continue;
      }
      return nullptr;
    }

    if (const auto *Phi = dyn_cast<PHINode>(V)) {
      for (const Value *Incoming : Phi->incoming_values())
        Push(Incoming, {});
      continue;
    }
    if (const auto *Select = dyn_cast<SelectInst>(V)) {
      Push(Select->getTrueValue(), {});
      Push(Select->getFalseValue(), {});
      continue;
    }
    if (const auto *Freeze = dyn_cast<FreezeInst>(V)) {
      Push(Freeze->getOperand(0), {});
      continue;
    }
    if (const auto *Extract = dyn_cast<ExtractValueInst>(V)) {
      Push(Extract->getAggregateOperand(), Extract->getIndices());
      continue;
    }
    if (const auto *GEP = dyn_cast<GetElementPtrInst>(V)) {
      Push(GEP->getPointerOperand(), {});
      continue;
    }
    if (const auto *Cast = dyn_cast<CastInst>(V);
        Cast && Cast->isNoopCast(Cast->getDataLayout())) {
      Push(Cast->getOperand(0), {});
      continue;
    }

    if (!isFixedFrameBase(V))
      return nullptr;
    if (Base && Base != V)
      return nullptr;
    Base = V;
  }

  // Reject an unanchored forwarding cycle and address-space changes which
  // cannot be reproduced by one Base+Offset recipe.
  return Base && Root->getType() == Base->getType() ? Base : nullptr;
}

Value *fixedFrameProvenanceBase(Value *V) {
  return const_cast<Value *>(
      fixedFrameProvenanceBase(static_cast<const Value *>(V)));
}

bool isFixedFrameAddress(const Value *V) {
  return V->getType()->isPointerTy() && fixedFrameProvenanceBase(V) != nullptr;
}

// LLVM's RewriteStatepointsForGC traces a live derived pointer to a base
// defining value (BDV). Constants have the non-moving null base, while a
// pointer PHI/select whose incoming values resolve to different bases gets a
// parallel base PHI/select. Keep the scalar part of that contract here: Go's
// stack maps carry only the inferred base, and an integer offset lets the
// existing rematerialization path rebuild the derived value after stack
// growth.
class ScalarPointerBaseAnalysis {
public:
  explicit ScalarPointerBaseAnalysis(Function &F) : DL(F.getDataLayout()) {}

  Expected<Value *> get(Value *V) {
    Expected<Value *> DefOrErr = findBaseOrBDV(V);
    if (!DefOrErr)
      return DefOrErr.takeError();
    Value *Def = *DefOrErr;
    if (isKnownBase(Def))
      return Def;

    MapVector<Value *, BaseState> States;
    SmallVector<Value *, 16> Worklist{Def};
    States.insert({Def, BaseState()});
    while (!Worklist.empty()) {
      Value *Current = Worklist.pop_back_val();
      Error Err = visitMergeOperands(Current, [&](Value *Input) -> Error {
        Expected<Value *> InputDefOrErr = findBaseOrBDV(Input);
        if (!InputDefOrErr)
          return InputDefOrErr.takeError();
        Value *InputDef = *InputDefOrErr;
        if (isKnownBase(InputDef))
          return Error::success();
        if (!isa<PHINode, SelectInst>(InputDef))
          return createStringError(
              std::errc::not_supported,
              "GoALLC pointer base analysis found an unsupported merge");
        if (States.insert({InputDef, BaseState()}).second)
          Worklist.push_back(InputDef);
        return Error::success();
      });
      if (Err)
        return std::move(Err);
    }

    // A merge of values which are already bases is itself a base. This is the
    // important distinction between `phi(null, frame)` and
    // `phi(null+C, frame+C)`.
    bool Changed;
    do {
      Changed = false;
      SmallVector<Value *, 8> BaseMerges;
      for (auto &[Merge, State] : States) {
        Value *CurrentMerge = Merge;
        bool AllInputsAreBases = true;
        Error Err = visitMergeOperands(Merge, [&](Value *Input) -> Error {
          if (Input->stripPointerCasts() == CurrentMerge)
            return Error::success();
          Expected<Value *> InputDefOrErr = findBaseOrBDV(Input);
          if (!InputDefOrErr)
            return InputDefOrErr.takeError();
          Value *InputDef = *InputDefOrErr;
          if (Input->stripPointerCasts() != InputDef || States.count(InputDef))
            AllInputsAreBases = false;
          return Error::success();
        });
        if (Err)
          return std::move(Err);
        if (AllInputsAreBases)
          BaseMerges.push_back(Merge);
      }
      for (Value *Merge : BaseMerges) {
        States.erase(Merge);
        ResolvedBases[Merge] = Merge;
        KnownBases[Merge] = true;
        Changed = true;
      }
    } while (Changed);

    if (!States.count(Def))
      return Def;

    bool Progress;
    do {
      Progress = false;
      for (auto &[Merge, OldState] : States) {
        BaseState NewState;
        Error Err = visitMergeOperands(Merge, [&](Value *Input) -> Error {
          Expected<Value *> InputDefOrErr = findBaseOrBDV(Input);
          if (!InputDefOrErr)
            return InputDefOrErr.takeError();
          Value *InputDef = *InputDefOrErr;
          auto It = States.find(InputDef);
          NewState.meet(It == States.end()
                            ? BaseState(BaseState::Base, InputDef)
                            : It->second);
          return Error::success();
        });
        if (Err)
          return std::move(Err);
        if (NewState != OldState) {
          OldState = NewState;
          Progress = true;
        }
      }
    } while (Progress);

    for (auto &[Merge, State] : States)
      if (State.Kind == BaseState::Unknown)
        return createStringError(
            std::errc::not_supported,
            "GoALLC pointer base analysis found an unanchored merge cycle");

    for (auto &[Merge, State] : States) {
      if (State.Kind != BaseState::Conflict)
        continue;
      Instruction *I = cast<Instruction>(Merge);
      Instruction *BaseMerge = nullptr;
      if (auto *Phi = dyn_cast<PHINode>(I)) {
        BaseMerge =
            PHINode::Create(Phi->getType(), Phi->getNumIncomingValues(),
                            Phi->getName() + ".base", Phi->getIterator());
      } else {
        BaseMerge = I->clone();
        BaseMerge->insertBefore(I->getIterator());
        BaseMerge->setName(I->getName() + ".base");
      }
      State.BaseValue = BaseMerge;
      DefiningValues[BaseMerge] = BaseMerge;
      ResolvedBases[BaseMerge] = BaseMerge;
      KnownBases[BaseMerge] = true;
    }

    for (auto &[Merge, State] : States) {
      if (State.Kind != BaseState::Conflict)
        continue;
      Instruction *BaseMerge = cast<Instruction>(State.BaseValue);
      auto BaseForInput = [&](Value *Input) -> Expected<Value *> {
        Expected<Value *> InputDefOrErr = findBaseOrBDV(Input);
        if (!InputDefOrErr)
          return InputDefOrErr.takeError();
        Value *InputDef = *InputDefOrErr;
        auto It = States.find(InputDef);
        Value *Base = It == States.end() ? InputDef : It->second.BaseValue;
        if (!Base || Base->getType() != Input->getType())
          return createStringError(
              std::errc::not_supported,
              "GoALLC pointer base has an incompatible scalar type");
        return Base;
      };

      if (auto *BasePhi = dyn_cast<PHINode>(BaseMerge)) {
        auto *Phi = cast<PHINode>(Merge);
        for (unsigned I = 0; I != Phi->getNumIncomingValues(); ++I) {
          Expected<Value *> BaseOrErr = BaseForInput(Phi->getIncomingValue(I));
          if (!BaseOrErr)
            return BaseOrErr.takeError();
          BasePhi->addIncoming(*BaseOrErr, Phi->getIncomingBlock(I));
        }
      } else {
        auto *BaseSelect = cast<SelectInst>(BaseMerge);
        auto *Select = cast<SelectInst>(Merge);
        Expected<Value *> TrueBaseOrErr = BaseForInput(Select->getTrueValue());
        if (!TrueBaseOrErr)
          return TrueBaseOrErr.takeError();
        Expected<Value *> FalseBaseOrErr =
            BaseForInput(Select->getFalseValue());
        if (!FalseBaseOrErr)
          return FalseBaseOrErr.takeError();
        BaseSelect->setTrueValue(*TrueBaseOrErr);
        BaseSelect->setFalseValue(*FalseBaseOrErr);
      }
    }

    deduplicateBaseMerges(States);
    for (auto &[Merge, State] : States)
      ResolvedBases[Merge] = State.BaseValue;
    return ResolvedBases.lookup(Def);
  }

private:
  struct BaseState {
    enum KindTy { Unknown, Base, Conflict } Kind = Unknown;
    Value *BaseValue = nullptr;

    BaseState() = default;
    BaseState(KindTy Kind, Value *BaseValue)
        : Kind(Kind), BaseValue(BaseValue) {}

    void meet(const BaseState &Other) {
      if (Kind == Conflict || Other.Kind == Unknown)
        return;
      if (Kind == Unknown) {
        *this = Other;
        return;
      }
      if (Other.Kind == Conflict || BaseValue != Other.BaseValue) {
        Kind = Conflict;
        BaseValue = nullptr;
      }
    }

    bool operator!=(const BaseState &Other) const {
      return Kind != Other.Kind || BaseValue != Other.BaseValue;
    }
  };

  const DataLayout &DL;
  MapVector<Value *, Value *> DefiningValues;
  MapVector<Value *, Value *> ResolvedBases;
  MapVector<Value *, bool> KnownBases;

  bool isKnownBase(Value *V) const {
    auto It = KnownBases.find(V);
    return It != KnownBases.end() && It->second;
  }

  Expected<Value *> findBaseDefiningValue(Value *V) {
    if (!V->getType()->isPointerTy())
      return createStringError(
          std::errc::not_supported,
          "GoALLC scalar pointer base has non-pointer type");
    if (auto It = DefiningValues.find(V); It != DefiningValues.end())
      return It->second;

    auto RememberBase = [&](Value *Base) -> Value * {
      DefiningValues[V] = Base;
      KnownBases[Base] = true;
      return Base;
    };
    if (isa<Constant>(V)) {
      auto *Null = ConstantPointerNull::get(cast<PointerType>(V->getType()));
      return RememberBase(Null);
    }
    if (isFixedFrameBase(V) ||
        isa<Argument, LoadInst, CallBase, IntToPtrInst, ExtractValueInst>(V))
      return RememberBase(V);
    if (auto *GEP = dyn_cast<GetElementPtrInst>(V)) {
      Expected<Value *> BaseOrErr =
          findBaseDefiningValue(GEP->getPointerOperand());
      if (!BaseOrErr)
        return BaseOrErr.takeError();
      DefiningValues[V] = *BaseOrErr;
      return *BaseOrErr;
    }
    if (auto *Freeze = dyn_cast<FreezeInst>(V)) {
      Expected<Value *> BaseOrErr =
          findBaseDefiningValue(Freeze->getOperand(0));
      if (!BaseOrErr)
        return BaseOrErr.takeError();
      DefiningValues[V] = *BaseOrErr;
      return *BaseOrErr;
    }
    if (auto *Cast = dyn_cast<CastInst>(V);
        Cast && Cast->getSrcTy()->isPointerTy() &&
        Cast->getDestTy()->isPointerTy() && Cast->isNoopCast(DL)) {
      Expected<Value *> BaseOrErr = findBaseDefiningValue(Cast->getOperand(0));
      if (!BaseOrErr)
        return BaseOrErr.takeError();
      DefiningValues[V] = *BaseOrErr;
      return *BaseOrErr;
    }
    if (isa<PHINode, SelectInst>(V)) {
      DefiningValues[V] = V;
      KnownBases[V] = false;
      return V;
    }

    // The current non-moving heap contract treats any other scalar pointer
    // producer as a base. Only transparent address derivations and merge nodes
    // participate in base inference.
    return RememberBase(V);
  }

  Expected<Value *> findBaseOrBDV(Value *V) {
    Expected<Value *> DefOrErr = findBaseDefiningValue(V);
    if (!DefOrErr)
      return DefOrErr.takeError();
    if (Value *Resolved = ResolvedBases.lookup(*DefOrErr))
      return Resolved;
    return *DefOrErr;
  }

  template <typename VisitorT>
  Error visitMergeOperands(Value *V, VisitorT &&Visit) {
    if (auto *Phi = dyn_cast<PHINode>(V)) {
      for (Value *Input : Phi->incoming_values())
        if (Error Err = Visit(Input))
          return Err;
      return Error::success();
    }
    if (auto *Select = dyn_cast<SelectInst>(V)) {
      if (Error Err = Visit(Select->getTrueValue()))
        return Err;
      return Visit(Select->getFalseValue());
    }
    return createStringError(std::errc::not_supported,
                             "GoALLC pointer BDV is not a PHI or select");
  }

  void deduplicateBaseMerges(MapVector<Value *, BaseState> &States) {
    auto ReplaceBase = [&](Instruction *Old, Instruction *New) {
      Old->replaceAllUsesWith(New);
      for (auto &[Merge, State] : States)
        if (State.BaseValue == Old)
          State.BaseValue = New;
      KnownBases[New] = true;
      DefiningValues.erase(Old);
      ResolvedBases.erase(Old);
      KnownBases.erase(Old);
      Old->eraseFromParent();
    };

    for (auto &[Merge, State] : States) {
      auto *BasePhi = dyn_cast_or_null<PHINode>(State.BaseValue);
      if (!BasePhi)
        continue;
      for (PHINode &Other : BasePhi->getParent()->phis()) {
        if (&Other == BasePhi || Other.getType() != BasePhi->getType() ||
            Other.getNumIncomingValues() != BasePhi->getNumIncomingValues())
          continue;
        bool Same = true;
        for (unsigned I = 0; I != Other.getNumIncomingValues(); ++I)
          Same &= Other.getIncomingBlock(I) == BasePhi->getIncomingBlock(I) &&
                  Other.getIncomingValue(I) == BasePhi->getIncomingValue(I);
        if (Same) {
          ReplaceBase(BasePhi, &Other);
          break;
        }
      }
    }

    for (auto &[Merge, State] : States) {
      auto *BaseSelect = dyn_cast_or_null<SelectInst>(State.BaseValue);
      if (!BaseSelect)
        continue;
      for (Instruction &I : *BaseSelect->getParent()) {
        auto *Other = dyn_cast<SelectInst>(&I);
        if (!Other || Other == BaseSelect || !Other->comesBefore(BaseSelect))
          continue;
        if (Other->getType() == BaseSelect->getType() &&
            Other->getCondition() == BaseSelect->getCondition() &&
            Other->getTrueValue() == BaseSelect->getTrueValue() &&
            Other->getFalseValue() == BaseSelect->getFalseValue()) {
          ReplaceBase(BaseSelect, Other);
          break;
        }
      }
    }
  }
};

class ScalarPointerOffsetBuilder {
public:
  ScalarPointerOffsetBuilder(Function &F, ScalarPointerBaseAnalysis &Bases)
      : DL(F.getDataLayout()), Bases(Bases) {}

  Expected<Value *> get(Value *V) {
    if (Value *Offset = Offsets.lookup(V))
      return Offset;
    Expected<Value *> BaseOrErr = Bases.get(V);
    if (!BaseOrErr)
      return BaseOrErr.takeError();
    Value *Base = *BaseOrErr;
    Type *IndexTy = DL.getIndexType(V->getType());
    if (V == Base) {
      Value *Zero = ConstantInt::get(IndexTy, 0);
      Offsets[V] = Zero;
      return Zero;
    }
    if (auto *C = dyn_cast<Constant>(V)) {
      if (DL.isNonIntegralPointerType(V->getType()))
        return createStringError(
            std::errc::not_supported,
            "GoALLC cannot form an offset for a non-integral pointer constant");
      Constant *Offset = ConstantExpr::getPtrToInt(C, IndexTy);
      Offset = ConstantFoldConstant(Offset, DL);
      Offsets[V] = Offset;
      return Offset;
    }
    if (auto *GEP = dyn_cast<GetElementPtrInst>(V)) {
      Expected<Value *> ParentOrErr = get(GEP->getPointerOperand());
      if (!ParentOrErr)
        return ParentOrErr.takeError();
      IRBuilder<> Builder(GEP);
      Value *GEPOffset = emitGEPOffset(&Builder, DL, GEP);
      if (GEPOffset->getType() != IndexTy)
        GEPOffset = Builder.CreateSExtOrTrunc(GEPOffset, IndexTy,
                                              GEP->getName() + ".offset.cast");
      Value *Offset = GEPOffset;
      if (auto *Parent = dyn_cast<ConstantInt>(*ParentOrErr);
          !Parent || !Parent->isZero())
        Offset = Builder.CreateAdd(*ParentOrErr, GEPOffset,
                                   GEP->getName() + ".offset");
      Offsets[V] = Offset;
      return Offset;
    }
    if (auto *Cast = dyn_cast<CastInst>(V);
        Cast && Cast->getSrcTy()->isPointerTy() &&
        Cast->getDestTy()->isPointerTy() && Cast->isNoopCast(DL)) {
      Expected<Value *> OffsetOrErr = get(Cast->getOperand(0));
      if (!OffsetOrErr)
        return OffsetOrErr.takeError();
      Offsets[V] = *OffsetOrErr;
      return *OffsetOrErr;
    }
    if (auto *Freeze = dyn_cast<FreezeInst>(V)) {
      Expected<Value *> OperandOrErr = get(Freeze->getOperand(0));
      if (!OperandOrErr)
        return OperandOrErr.takeError();
      IRBuilder<> Builder(Freeze);
      Value *Offset =
          Builder.CreateFreeze(*OperandOrErr, Freeze->getName() + ".offset");
      Offsets[V] = Offset;
      return Offset;
    }
    if (auto *Phi = dyn_cast<PHINode>(V)) {
      auto *OffsetPhi =
          PHINode::Create(IndexTy, Phi->getNumIncomingValues(),
                          Phi->getName() + ".offset", Phi->getIterator());
      Offsets[V] = OffsetPhi;
      for (unsigned I = 0; I != Phi->getNumIncomingValues(); ++I) {
        Expected<Value *> IncomingOrErr = get(Phi->getIncomingValue(I));
        if (!IncomingOrErr)
          return IncomingOrErr.takeError();
        OffsetPhi->addIncoming(*IncomingOrErr, Phi->getIncomingBlock(I));
      }
      Value *Common = nullptr;
      for (Value *Incoming : OffsetPhi->incoming_values()) {
        if (Incoming == OffsetPhi)
          continue;
        if (!Common)
          Common = Incoming;
        else if (Common != Incoming)
          return OffsetPhi;
      }
      if (Common) {
        Offsets[V] = Common;
        OffsetPhi->replaceAllUsesWith(Common);
        OffsetPhi->eraseFromParent();
        return Common;
      }
      return OffsetPhi;
    }
    if (auto *Select = dyn_cast<SelectInst>(V)) {
      Expected<Value *> TrueOrErr = get(Select->getTrueValue());
      if (!TrueOrErr)
        return TrueOrErr.takeError();
      Expected<Value *> FalseOrErr = get(Select->getFalseValue());
      if (!FalseOrErr)
        return FalseOrErr.takeError();
      Value *Offset = *TrueOrErr;
      if (*TrueOrErr != *FalseOrErr) {
        IRBuilder<> Builder(Select);
        Offset =
            Builder.CreateSelect(Select->getCondition(), *TrueOrErr,
                                 *FalseOrErr, Select->getName() + ".offset");
      }
      Offsets[V] = Offset;
      return Offset;
    }
    return createStringError(
        std::errc::not_supported,
        "GoALLC cannot express a derived scalar pointer as base plus offset");
  }

private:
  const DataLayout &DL;
  ScalarPointerBaseAnalysis &Bases;
  MapVector<Value *, Value *> Offsets;
};

Error normalizeMergedDerivedPointers(Function &F) {
  ScalarPointerBaseAnalysis Bases(F);
  ScalarPointerOffsetBuilder Offsets(F, Bases);
  struct Normalization {
    WeakTrackingVH Address;
    WeakTrackingVH Base;
    WeakTrackingVH Offset;
  };
  SmallVector<WeakTrackingVH, 16> Candidates;
  for (Instruction &I : instructions(F))
    // Same-object fixed-frame merges already use FixedFrameOffsetBuilder and
    // must retain their canonical alloca/byval/goret identity.
    if (I.getType()->isPointerTy() && isa<PHINode, SelectInst>(I) &&
        !isFixedFrameAddress(&I))
      Candidates.push_back(&I);

  SmallVector<Normalization, 16> Normalizations;
  for (WeakTrackingVH &Handle : Candidates) {
    auto *I = dyn_cast_or_null<Instruction>(Handle);
    if (!I)
      continue;
    Expected<Value *> BaseOrErr = Bases.get(I);
    if (!BaseOrErr)
      return BaseOrErr.takeError();
    if (*BaseOrErr == I)
      continue;
    Expected<Value *> OffsetOrErr = Offsets.get(I);
    if (!OffsetOrErr)
      return OffsetOrErr.takeError();
    Normalizations.push_back({I, *BaseOrErr, *OffsetOrErr});
  }

  SmallVector<WeakTrackingVH, 16> Dead;
  for (const Normalization &Normalization : Normalizations) {
    auto *Address = dyn_cast_or_null<Instruction>(Normalization.Address);
    Value *Base = Normalization.Base;
    Value *Offset = Normalization.Offset;
    if (!Address || !Base || !Offset)
      return createStringError(
          std::errc::invalid_argument,
          "GoALLC derived pointer normalization lost an SSA value");
    Instruction *InsertBefore = Address;
    if (auto *Phi = dyn_cast<PHINode>(Address))
      InsertBefore = &*Phi->getParent()->getFirstNonPHIIt();
    IRBuilder<> Builder(InsertBefore);
    Builder.SetCurrentDebugLocation(Address->getDebugLoc());
    Value *Derived =
        Builder.CreateGEP(Builder.getInt8Ty(), Base, Offset,
                          Address->hasName() ? Address->getName() + ".derived"
                                             : "derived.pointer");
    Address->replaceAllUsesWith(Derived);
    Dead.push_back(Address);
  }
  for (WeakTrackingVH &Handle : llvm::reverse(Dead)) {
    auto *I = dyn_cast_or_null<Instruction>(Handle);
    if (!I)
      continue;
    if (auto *Phi = dyn_cast<PHINode>(I))
      RecursivelyDeleteDeadPHINode(Phi);
    else
      RecursivelyDeleteTriviallyDeadInstructions(I);
  }
  return Error::success();
}

const Value *rematerializableDerivedBase(const Value *V) {
  if (!isRelocatablePointerType(V->getType()) || isFixedFrameAddress(V))
    return nullptr;

  bool HasDerivedOperation = false;
  while (true) {
    if (const auto *GEP = dyn_cast<GetElementPtrInst>(V)) {
      V = GEP->getPointerOperand();
      HasDerivedOperation = true;
      continue;
    }
    if (const auto *Cast = dyn_cast<CastInst>(V);
        Cast && isRelocatablePointerType(Cast->getSrcTy()) &&
        isRelocatablePointerType(Cast->getDestTy()) &&
        Cast->isNoopCast(Cast->getDataLayout())) {
      V = Cast->getOperand(0);
      HasDerivedOperation = true;
      continue;
    }
    break;
  }

  // Constants cannot be relocated. Leave addresses rooted at them on the
  // ordinary pointer path until their provenance has an explicit
  // representation.
  if (!HasDerivedOperation || isa<Constant>(V) || isa<AllocaInst>(V) ||
      !isRelocatablePointerType(V->getType()))
    return nullptr;
  return V;
}

Value *rematerializableDerivedBase(Value *V) {
  return const_cast<Value *>(
      rematerializableDerivedBase(static_cast<const Value *>(V)));
}

FrameAddressUseKind classifyFrameAddressUse(const Use &U) {
  auto *I = dyn_cast<Instruction>(U.getUser());
  if (!I)
    return FrameAddressUseKind::FirstClass;
  if (isa<GetElementPtrInst, BitCastInst, AddrSpaceCastInst>(I))
    return FrameAddressUseKind::Derivation;
  if (auto *Load = dyn_cast<LoadInst>(I))
    return &U == &Load->getOperandUse(LoadInst::getPointerOperandIndex())
               ? FrameAddressUseKind::TerminalMemory
               : FrameAddressUseKind::FirstClass;
  if (auto *Store = dyn_cast<StoreInst>(I))
    return &U == &Store->getOperandUse(StoreInst::getPointerOperandIndex())
               ? FrameAddressUseKind::TerminalMemory
               : FrameAddressUseKind::FirstClass;
  if (auto *RMW = dyn_cast<AtomicRMWInst>(I))
    return &U == &RMW->getOperandUse(AtomicRMWInst::getPointerOperandIndex())
               ? FrameAddressUseKind::TerminalMemory
               : FrameAddressUseKind::FirstClass;
  if (auto *CmpXchg = dyn_cast<AtomicCmpXchgInst>(I))
    return &U == &CmpXchg->getOperandUse(
                     AtomicCmpXchgInst::getPointerOperandIndex())
               ? FrameAddressUseKind::TerminalMemory
               : FrameAddressUseKind::FirstClass;
  if (auto *Mem = dyn_cast<MemIntrinsic>(I)) {
    if (U.get() == Mem->getRawDest())
      return FrameAddressUseKind::TerminalMemory;
    if (auto *Transfer = dyn_cast<MemTransferInst>(Mem);
        Transfer && U.get() == Transfer->getRawSource())
      return FrameAddressUseKind::TerminalMemory;
  }
  if (auto *Intrinsic = dyn_cast<IntrinsicInst>(I)) {
    if (Intrinsic->isLifetimeStartOrEnd() || isa<DbgInfoIntrinsic>(Intrinsic))
      return FrameAddressUseKind::LifetimeOrDebug;
    if (Intrinsic->getIntrinsicID() == Intrinsic::fake_use)
      return FrameAddressUseKind::FakeUse;
  }
  return FrameAddressUseKind::FirstClass;
}

// Walk the canonical pointer SSA closure for one fixed frame object. Direct
// GEP/cast recipes and same-object PHI/select/freeze forwarding stay inside the
// closure; callers receive only terminal, bookkeeping, escaping, or ambiguous
// uses and can apply their own content-liveness policy.
template <typename VisitorT>
void visitFixedFrameAddressUses(Value &Base, VisitorT &&Visit) {
  SmallVector<Value *, 16> Worklist{&Base};
  SmallPtrSet<Value *, 16> Seen;
  while (!Worklist.empty()) {
    Value *Address = Worklist.pop_back_val();
    if (!Seen.insert(Address).second)
      continue;
    for (Use &U : Address->uses()) {
      auto *I = dyn_cast<Instruction>(U.getUser());
      FrameAddressUseKind Kind = classifyFrameAddressUse(U);
      bool IsForwarding = I && I->getType()->isPointerTy() &&
                          (Kind == FrameAddressUseKind::Derivation ||
                           (Kind == FrameAddressUseKind::FirstClass &&
                            isa<PHINode, SelectInst, FreezeInst>(I))) &&
                          fixedFrameProvenanceBase(I) == &Base;
      if (IsForwarding) {
        Worklist.push_back(I);
        continue;
      }
      Visit(Address, U, I, Kind);
    }
  }
}

Value *rematerializeAddress(Value *Address, Value *Base, Value *RelocatedBase,
                            Instruction *InsertBefore);

struct FixedFrameAddressRecord {
  Value *Address;
  Value *Base;
  // Integer offsets may themselves be ordinary call results. Statepoint
  // rewriting replaces such a call with gc.result before addresses are
  // localized, so retain a tracking handle rather than a raw pointer to the
  // erased call instruction.
  WeakTrackingVH Offset;
};

struct FixedFrameOffsetCacheEntry {
  Value *Base;
  Value *V;
  SmallVector<unsigned, 4> Indices;
  Value *Offset;
};

// Convert a fixed-frame pointer SSA graph into an integer offset graph. The
// integer graph may cross a statepoint because stack movement changes only the
// object's physical base. Pointer PHIs/selects therefore become integer
// PHIs/selects, including the different-offset case, while one canonical frame
// base remains the only gc-live identity.
class FixedFrameOffsetBuilder {
public:
  explicit FixedFrameOffsetBuilder(Function &F) : DL(F.getDataLayout()) {}

  Expected<Value *> get(Value *V, Value *Base) {
    return get(V, Base, ArrayRef<unsigned>());
  }

private:
  const DataLayout &DL;
  SmallVector<FixedFrameOffsetCacheEntry, 32> Cache;

  Value *lookup(Value *V, Value *Base, ArrayRef<unsigned> Indices) {
    auto It =
        llvm::find_if(Cache, [&](const FixedFrameOffsetCacheEntry &Entry) {
          return Entry.Base == Base && Entry.V == V && Entry.Indices == Indices;
        });
    return It == Cache.end() ? nullptr : It->Offset;
  }

  void remember(Value *V, Value *Base, ArrayRef<unsigned> Indices,
                Value *Offset) {
    Cache.push_back({Base, V, SmallVector<unsigned, 4>(Indices), Offset});
  }

  void update(Value *V, Value *Base, ArrayRef<unsigned> Indices,
              Value *Offset) {
    auto It =
        llvm::find_if(Cache, [&](const FixedFrameOffsetCacheEntry &Entry) {
          return Entry.Base == Base && Entry.V == V && Entry.Indices == Indices;
        });
    assert(It != Cache.end() && "fixed-frame offset cache entry is missing");
    It->Offset = Offset;
  }

  Value *foldUniformPHI(PHINode *Phi, Value *V, Value *Base,
                        ArrayRef<unsigned> Indices) {
    if (Phi->getNumIncomingValues() == 0)
      return Phi;
    Value *Common = Phi->getIncomingValue(0);
    if (Common == Phi ||
        !llvm::all_of(Phi->incoming_values(),
                      [&](Value *Incoming) { return Incoming == Common; }))
      return Phi;
    Phi->replaceAllUsesWith(Common);
    Phi->eraseFromParent();
    update(V, Base, Indices, Common);
    return Common;
  }

  Expected<Value *> get(Value *V, Value *Base, ArrayRef<unsigned> Indices) {
    if (Value *Offset = lookup(V, Base, Indices))
      return Offset;

    Type *IndexTy = DL.getIndexType(Base->getType());
    auto Name = [&]() -> Twine {
      return V->hasName() ? V->getName() + ".offset" : "fixed.frame.offset";
    };

    if (Indices.empty() && V == Base) {
      Value *Zero = ConstantInt::get(IndexTy, 0);
      remember(V, Base, Indices, Zero);
      return Zero;
    }

    // PHI/select/freeze forwarding is identical for a scalar pointer and for
    // one pointer leaf selected from an aggregate. Carry Indices through the
    // forwarding node and resolve the concrete leaf only at insert/extract.
    if (auto *Phi = dyn_cast<PHINode>(V)) {
      auto *OffsetPhi = PHINode::Create(IndexTy, Phi->getNumIncomingValues(),
                                        Name(), Phi->getIterator());
      remember(V, Base, Indices, OffsetPhi);
      for (unsigned I = 0; I != Phi->getNumIncomingValues(); ++I) {
        Expected<Value *> Incoming =
            get(Phi->getIncomingValue(I), Base, Indices);
        if (!Incoming)
          return Incoming.takeError();
        OffsetPhi->addIncoming(*Incoming, Phi->getIncomingBlock(I));
      }
      return foldUniformPHI(OffsetPhi, V, Base, Indices);
    }
    if (auto *Select = dyn_cast<SelectInst>(V)) {
      Expected<Value *> TrueOffset = get(Select->getTrueValue(), Base, Indices);
      if (!TrueOffset)
        return TrueOffset.takeError();
      Expected<Value *> FalseOffset =
          get(Select->getFalseValue(), Base, Indices);
      if (!FalseOffset)
        return FalseOffset.takeError();
      IRBuilder<> Builder(Select);
      Value *Offset =
          *TrueOffset == *FalseOffset
              ? *TrueOffset
              : Builder.CreateSelect(Select->getCondition(), *TrueOffset,
                                     *FalseOffset, Name());
      remember(V, Base, Indices, Offset);
      return Offset;
    }
    if (auto *Freeze = dyn_cast<FreezeInst>(V)) {
      Expected<Value *> OperandOffset =
          get(Freeze->getOperand(0), Base, Indices);
      if (!OperandOffset)
        return OperandOffset.takeError();
      IRBuilder<> Builder(Freeze);
      Value *Offset = Builder.CreateFreeze(*OperandOffset, Name());
      remember(V, Base, Indices, Offset);
      return Offset;
    }
    if (auto *Extract = dyn_cast<ExtractValueInst>(V)) {
      SmallVector<unsigned, 4> Combined(Extract->getIndices());
      Combined.append(Indices.begin(), Indices.end());
      Expected<Value *> Offset =
          get(Extract->getAggregateOperand(), Base, Combined);
      if (!Offset)
        return Offset.takeError();
      remember(V, Base, Indices, *Offset);
      return *Offset;
    }

    if (!Indices.empty()) {
      if (auto *Insert = dyn_cast<InsertValueInst>(V)) {
        ArrayRef<unsigned> InsertIndices = Insert->getIndices();
        Expected<Value *> Offset = [&]() -> Expected<Value *> {
          if (Indices.size() >= InsertIndices.size() &&
              llvm::equal(InsertIndices,
                          Indices.take_front(InsertIndices.size())))
            return get(Insert->getInsertedValueOperand(), Base,
                       Indices.drop_front(InsertIndices.size()));
          return get(Insert->getAggregateOperand(), Base, Indices);
        }();
        if (!Offset)
          return Offset.takeError();
        remember(V, Base, Indices, *Offset);
        return *Offset;
      }
      if (Value *Inserted = FindInsertedValue(V, Indices);
          Inserted && Inserted != V) {
        Expected<Value *> Offset = get(Inserted, Base, {});
        if (!Offset)
          return Offset.takeError();
        remember(V, Base, Indices, *Offset);
        return *Offset;
      }
      return createStringError(
          std::errc::not_supported,
          "GoALLC cannot normalize a fixed-frame aggregate pointer leaf");
    }
    if (auto *GEP = dyn_cast<GetElementPtrInst>(V)) {
      Expected<Value *> ParentOffset = get(GEP->getPointerOperand(), Base, {});
      if (!ParentOffset)
        return ParentOffset.takeError();
      IRBuilder<> Builder(GEP);
      Value *GEPOffset = emitGEPOffset(&Builder, DL, GEP);
      if (GEPOffset->getType() != IndexTy) {
        if (!GEPOffset->getType()->isIntegerTy() || !IndexTy->isIntegerTy())
          return createStringError(
              std::errc::not_supported,
              "GoALLC fixed-frame GEP has an incompatible offset type");
        GEPOffset = Builder.CreateSExtOrTrunc(GEPOffset, IndexTy,
                                              GEP->getName() + ".offset.cast");
      }
      Value *Offset = GEPOffset;
      auto *ParentConstant = dyn_cast<ConstantInt>(*ParentOffset);
      if (!ParentConstant || !ParentConstant->isZero())
        Offset = Builder.CreateAdd(*ParentOffset, GEPOffset, Name());
      remember(V, Base, Indices, Offset);
      return Offset;
    }
    if (auto *Cast = dyn_cast<CastInst>(V); Cast && Cast->isNoopCast(DL)) {
      Expected<Value *> Offset = get(Cast->getOperand(0), Base, {});
      if (!Offset)
        return Offset.takeError();
      remember(V, Base, Indices, *Offset);
      return *Offset;
    }

    return createStringError(
        std::errc::not_supported,
        "GoALLC cannot normalize a fixed-frame pointer offset");
  }
};

Expected<SmallVector<FixedFrameAddressRecord, 32>>
prepareFixedFrameAddresses(Function &F) {
  ValueSet Candidates;
  for (Argument &Arg : F.args())
    if (isFixedFrameArgument(&Arg))
      Candidates.insert(&Arg);
  for (Instruction &I : instructions(F))
    if (I.getType()->isPointerTy() && fixedFrameProvenanceBase(&I))
      Candidates.insert(&I);

  FixedFrameOffsetBuilder Builder(F);
  SmallVector<FixedFrameAddressRecord, 32> Records;
  Records.reserve(Candidates.size());
  for (Value *Address : Candidates) {
    Value *Base = fixedFrameProvenanceBase(Address);
    assert(Base && "fixed-frame candidate lost its provenance");
    Expected<Value *> Offset = Builder.get(Address, Base);
    if (!Offset)
      return Offset.takeError();
    Records.push_back({Address, Base, *Offset});
  }
  return Records;
}

Error localizeFixedFrameAddresses(ArrayRef<FixedFrameAddressRecord> Records) {
  SmallVector<WeakTrackingVH, 32> DeadAddresses;
  for (const FixedFrameAddressRecord &Record : Records) {
    Value *Address = Record.Address;
    Value *Offset = Record.Offset;
    if (!Offset)
      return createStringError(
          std::errc::invalid_argument,
          "GoALLC fixed-frame offset was deleted during statepoint rewrite");
    // A raw alloca/byval/goret value is already an abstract frame identity.
    // SelectionDAG selects its FrameIndex independently in every block (with
    // the Go fixed-argument hook for byval/goret), so only derived addresses
    // need an explicit Base+Offset reconstruction.
    if (Address == Record.Base)
      continue;
    MapVector<BasicBlock *, Instruction *> InsertPoints;
    SmallVector<Use *, 8> Uses;
    for (Use &U : Address->uses()) {
      auto *User = dyn_cast<Instruction>(U.getUser());
      if (!User)
        return createStringError(
            std::errc::not_supported,
            "GoALLC cannot localize a non-instruction fixed-frame use");

      FrameAddressUseKind Kind = classifyFrameAddressUse(U);
      if (Kind == FrameAddressUseKind::Derivation ||
          Kind == FrameAddressUseKind::LifetimeOrDebug)
        continue;

      Instruction *InsertBefore = User;
      if (auto *Phi = dyn_cast<PHINode>(User))
        InsertBefore = Phi->getIncomingBlock(U)->getTerminator();
      BasicBlock *BB = InsertBefore->getParent();
      Instruction *&Earliest = InsertPoints[BB];
      if (!Earliest || InsertBefore->comesBefore(Earliest))
        Earliest = InsertBefore;
      Uses.push_back(&U);
    }

    DenseMap<BasicBlock *, Value *> LocalAddresses;
    for (const auto &[BB, InsertBefore] : InsertPoints) {
      IRBuilder<> Builder(InsertBefore);
      Builder.SetCurrentDebugLocation(InsertBefore->getDebugLoc());
      Value *Local =
          Builder.CreateGEP(Builder.getInt8Ty(), Record.Base, Offset,
                            Address->hasName() ? Address->getName() + ".remat"
                                               : "fixed.frame.address");
      LocalAddresses[BB] = Local;
    }

    for (Use *U : Uses) {
      auto *User = cast<Instruction>(U->getUser());
      BasicBlock *BB = User->getParent();
      if (auto *Phi = dyn_cast<PHINode>(User))
        BB = Phi->getIncomingBlock(*U);
      Value *Local = LocalAddresses.lookup(BB);
      assert(Local && "fixed-frame use has no block-local address");
      U->set(Local);
    }

    if (auto *I = dyn_cast<Instruction>(Address); I && !isa<AllocaInst>(I))
      DeadAddresses.push_back(I);
  }

  // The offset graph no longer depends on these pointer recipes. Remove dead
  // GEP/cast/PHI/select chains after every concrete use has its block-local
  // Base+Offset materialization.
  for (WeakTrackingVH &Handle : llvm::reverse(DeadAddresses)) {
    auto *I = dyn_cast_or_null<Instruction>(Handle);
    if (!I)
      continue;
    if (auto *Phi = dyn_cast<PHINode>(I))
      RecursivelyDeleteDeadPHINode(Phi);
    else
      RecursivelyDeleteTriviallyDeadInstructions(I);
  }
  return Error::success();
}

// The frontend attaches the marker only to raw inttoptr instructions. Follow
// transparent pointer derivations so their liveness has the same unmanaged
// semantics. The two walks deliberately separate "reaches a marker" from
// "contains only marked inputs": a self-referential loop PHI is accepted when
// anchored by a marker, but an otherwise unanchored pointer cycle is not.
bool hasNotInHeapAddressMarker(const Value *V,
                               SmallPtrSetImpl<const Value *> &Active) {
  if (const auto *I = dyn_cast<Instruction>(V);
      I && I->getMetadata(GoNotInHeapAddressMD))
    return true;
  if (!V->getType()->isPointerTy() || !Active.insert(V).second)
    return false;

  bool Result = false;
  if (const auto *GEP = dyn_cast<GetElementPtrInst>(V))
    Result = hasNotInHeapAddressMarker(GEP->getPointerOperand(), Active);
  else if (const auto *Freeze = dyn_cast<FreezeInst>(V))
    Result = hasNotInHeapAddressMarker(Freeze->getOperand(0), Active);
  else if (const auto *Cast = dyn_cast<CastInst>(V);
           Cast && Cast->getSrcTy()->isPointerTy() &&
           Cast->getDestTy()->isPointerTy())
    Result = hasNotInHeapAddressMarker(Cast->getOperand(0), Active);
  else if (const auto *Phi = dyn_cast<PHINode>(V))
    Result = llvm::any_of(Phi->incoming_values(), [&](const Value *Incoming) {
      return hasNotInHeapAddressMarker(Incoming, Active);
    });
  else if (const auto *Select = dyn_cast<SelectInst>(V))
    Result = hasNotInHeapAddressMarker(Select->getTrueValue(), Active) ||
             hasNotInHeapAddressMarker(Select->getFalseValue(), Active);

  Active.erase(V);
  return Result;
}

bool hasOnlyNotInHeapAddressInputs(const Value *V,
                                   SmallPtrSetImpl<const Value *> &Active) {
  if (const auto *I = dyn_cast<Instruction>(V);
      I && I->getMetadata(GoNotInHeapAddressMD))
    return true;
  if (isa<Constant>(V))
    return true;
  if (!V->getType()->isPointerTy())
    return false;
  if (!Active.insert(V).second)
    return true;

  bool Result = false;
  if (const auto *GEP = dyn_cast<GetElementPtrInst>(V))
    Result = hasOnlyNotInHeapAddressInputs(GEP->getPointerOperand(), Active);
  else if (const auto *Freeze = dyn_cast<FreezeInst>(V))
    Result = hasOnlyNotInHeapAddressInputs(Freeze->getOperand(0), Active);
  else if (const auto *Cast = dyn_cast<CastInst>(V);
           Cast && Cast->getSrcTy()->isPointerTy() &&
           Cast->getDestTy()->isPointerTy())
    Result = hasOnlyNotInHeapAddressInputs(Cast->getOperand(0), Active);
  else if (const auto *Phi = dyn_cast<PHINode>(V))
    Result = llvm::all_of(Phi->incoming_values(), [&](const Value *Incoming) {
      return hasOnlyNotInHeapAddressInputs(Incoming, Active);
    });
  else if (const auto *Select = dyn_cast<SelectInst>(V))
    Result = hasOnlyNotInHeapAddressInputs(Select->getTrueValue(), Active) &&
             hasOnlyNotInHeapAddressInputs(Select->getFalseValue(), Active);

  Active.erase(V);
  return Result;
}

bool isNotInHeapAddress(const Value *V) {
  if (!V->getType()->isPointerTy())
    return false;
  SmallPtrSet<const Value *, 16> MarkerActive;
  if (!hasNotInHeapAddressMarker(V, MarkerActive))
    return false;
  SmallPtrSet<const Value *, 16> InputActive;
  return hasOnlyNotInHeapAddressInputs(V, InputActive);
}

bool isStatepointValue(const Value *V) {
  if (isa<Constant>(V))
    return false;
  if (isNotInHeapAddress(V))
    return false;
  return containsPointer(V->getType());
}

bool isPointerAggregateValue(const Value *V) {
  Type *Ty = V->getType();
  return !isRelocatablePointerType(Ty) && containsPointer(Ty);
}

bool isOrdinaryRelocatablePointer(const Value *V) {
  return isRelocatablePointerType(V->getType()) && !isFixedFrameAddress(V) &&
         !rematerializableDerivedBase(V);
}

void addLiveValue(Value *V, ValueSet &Live) {
  if (isStatepointValue(V))
    Live.insert(V);
}

bool isLeafCall(const CallBase &Call) {
  if (Call.hasFnAttr(GCLeafAttr))
    return true;
  if (const Function *Callee = Call.getCalledFunction())
    return Callee->isIntrinsic() || Callee->hasFnAttribute(GCLeafAttr);
  return false;
}

// A terminal tail transfer leaves the current frame before entering the
// callee, so no pointer can remain live in this frame across the call. Check
// the structure as well as the tail marker: LLVM can preserve a tail marker
// after inlining even when the call is no longer in tail position.
bool isTerminalTailTransfer(const CallBase &Call) {
  const auto *Tail = dyn_cast<CallInst>(&Call);
  if (!Tail || !Tail->isTailCall())
    return false;
  const auto *Ret = dyn_cast_or_null<ReturnInst>(Tail->getNextNode());
  if (!Ret)
    return false;
  if (Tail->getType()->isVoidTy())
    return Ret->getReturnValue() == nullptr;
  return Ret->getReturnValue() == Tail;
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
                  BasicBlock::reverse_iterator End, ValueSet &Live) {
  for (Instruction &I : make_range(Begin, End)) {
    Live.remove(&I);
    if (isa<PHINode>(I))
      continue;
    for (Value *Operand : I.operands())
      addLiveValue(Operand, Live);
  }
}

void seedPhiUses(BasicBlock &BB, ValueSet &LiveOut) {
  for (BasicBlock *Succ : successors(&BB)) {
    for (Instruction &I : *Succ) {
      auto *Phi = dyn_cast<PHINode>(&I);
      if (!Phi)
        break;
      Value *Incoming = Phi->getIncomingValueForBlock(&BB);
      addLiveValue(Incoming, LiveOut);
    }
  }
}

LivenessData computeStatepointLiveness(Function &F) {
  LivenessData Data;
  SmallSetVector<BasicBlock *, 32> Worklist;

  for (BasicBlock &BB : F) {
    ValueSet &Kill = Data.Kill[&BB];
    for (Instruction &I : BB)
      if (isStatepointValue(&I))
        Kill.insert(&I);

    ValueSet &Gen = Data.Gen[&BB];
    scanBackward(BB.rbegin(), BB.rend(), Gen);

    ValueSet &Out = Data.LiveOut[&BB];
    seedPhiUses(BB, Out);

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

ValueSet liveAtCall(CallInst &Call, LivenessData &Data) {
  ValueSet Live = Data.LiveOut[Call.getParent()];
  // Go scans the callee's incoming arguments through
  // FUNCDATA_ArgsPointerMaps. The caller's statepoint therefore contains only
  // values live after the call, not values whose sole use is the call itself.
  scanBackward(Call.getParent()->rbegin(), Call.getIterator().getReverse(),
               Live);
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
  if (isa<FixedVectorType>(Ty)) {
    Leaves.push_back({SmallVector<unsigned, 4>(ArrayRef<unsigned>(Path))});
    return Error::success();
  }
  if (Ty->isVectorTy())
    return createStringError(std::errc::not_supported,
                             "GoALLC statepoints do not support scalable "
                             "vectors inside live pointer "
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

Value *findUniformUninitializedLeaf(Value &Aggregate,
                                    ArrayRef<unsigned> Indices,
                                    SmallPtrSetImpl<Value *> &Active) {
  if (Value *Inserted = FindInsertedValue(&Aggregate, Indices))
    return isa<UndefValue, PoisonValue>(Inserted) ? Inserted : nullptr;

  // FindInsertedValue deliberately stops at control-flow merges. Look through
  // a merge only when every incoming aggregate contributes the exact same
  // undef or poison leaf. Reusing that constant preserves the IR semantics and
  // prevents scalarization from manufacturing a pointer SSA value whose
  // SelectionDAG representation is only an IMPLICIT_DEF.
  if (!Active.insert(&Aggregate).second)
    return nullptr;

  SmallVector<Value *, 4> IncomingAggregates;
  if (auto *Phi = dyn_cast<PHINode>(&Aggregate)) {
    IncomingAggregates.append(Phi->incoming_values().begin(),
                              Phi->incoming_values().end());
  } else if (auto *Select = dyn_cast<SelectInst>(&Aggregate)) {
    IncomingAggregates.push_back(Select->getTrueValue());
    IncomingAggregates.push_back(Select->getFalseValue());
  }

  Value *Uniform = nullptr;
  for (Value *Incoming : IncomingAggregates) {
    Value *Leaf = findUniformUninitializedLeaf(*Incoming, Indices, Active);
    if (!Leaf || (Uniform && Leaf != Uniform)) {
      Uniform = nullptr;
      break;
    }
    Uniform = Leaf;
  }
  Active.erase(&Aggregate);
  return Uniform;
}

Value *findUniformUninitializedLeaf(Value &Aggregate,
                                    ArrayRef<unsigned> Indices) {
  SmallPtrSet<Value *, 8> Active;
  return findUniformUninitializedLeaf(Aggregate, Indices, Active);
}

Expected<SmallVector<Value *, 8>>
extractAggregateLeaves(Value &Aggregate, ArrayRef<AggregateLeaf> Leaves,
                       Function &F,
                       MapVector<Value *, Value *> &DirectPointerLeafSources) {
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
  for (const AggregateLeaf &Leaf : Leaves) {
    // Reuse alloca-derived inserted values so the post-scalarization fixed
    // frame canonicalization can rebuild their complete address recipe at
    // each concrete aggregate use. Other defined leaves retain distinct SSA
    // identities because their ordinary relocation repair must not rewrite
    // unrelated uses of the inserted value. Materializing an extractvalue
    // from undef or poison would invent an ordinary pointer root whose
    // SelectionDAG spill may be removed.
    Value *Inserted = FindInsertedValue(&Aggregate, Leaf.Indices);
    Value *LeafValue = nullptr;
    if (Inserted && (isa<UndefValue, PoisonValue>(Inserted) ||
                     isFixedFrameAddress(Inserted))) {
      LeafValue = Inserted;
    } else if (!Inserted) {
      LeafValue = findUniformUninitializedLeaf(Aggregate, Leaf.Indices);
    }
    if (!LeafValue) {
      LeafValue = Builder.CreateExtractValue(&Aggregate, Leaf.Indices,
                                             leafName(Aggregate, Leaf.Indices));
      // Keep the distinct extractvalue identity for relocation repair, but
      // remember exact insertvalue provenance. If the inserted pointer is also
      // independently live at a safepoint, both identities represent the same
      // root there and can share one gc.relocate.
      if (Inserted && LeafValue->getType()->isPointerTy() &&
          isOrdinaryRelocatablePointer(Inserted))
        DirectPointerLeafSources[LeafValue] = Inserted;
    }
    Values.push_back(LeafValue);
  }
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

Error reloadAggregateArgumentProjection(
    AllocaInst &Home, Type *AggregateType, ArrayRef<unsigned> Indices,
    Instruction &Projection,
    SmallVectorImpl<std::pair<Instruction *, unsigned>> &Recipe,
    ArrayRef<CallInst *> LiveCalls, DominatorTree &DT, LoopInfo &LI) {
  SmallVector<Use *, 8> Uses;
  for (Use &U : Projection.uses())
    Uses.push_back(&U);
  DenseMap<Instruction *, Value *> ReloadedAtUsePoint;

  for (Use *U : Uses) {
    if (auto *Nested = dyn_cast<ExtractValueInst>(U->getUser())) {
      SmallVector<unsigned, 4> NestedIndices(Indices);
      NestedIndices.append(Nested->idx_begin(), Nested->idx_end());
      if (Error Err = reloadAggregateArgumentProjection(
              Home, AggregateType, NestedIndices, *Nested, Recipe, LiveCalls,
              DT, LI))
        return Err;
      if (Nested->use_empty())
        Nested->eraseFromParent();
      continue;
    }
    if (auto *GEP = dyn_cast<GetElementPtrInst>(U->getUser());
        GEP &&
        U->getOperandNo() == GetElementPtrInst::getPointerOperandIndex()) {
      Recipe.push_back({GEP, U->getOperandNo()});
      if (Error Err = reloadAggregateArgumentProjection(
              Home, AggregateType, Indices, *GEP, Recipe, LiveCalls, DT, LI))
        return Err;
      Recipe.pop_back();
      if (GEP->use_empty())
        GEP->eraseFromParent();
      continue;
    }

    Instruction *InsertBefore = aggregateUseInsertionPoint(*U);
    if (!InsertBefore)
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints cannot reload a non-instruction aggregate "
          "argument projection");
    if (llvm::none_of(LiveCalls, [&](CallInst *Call) {
          return isPotentiallyReachable(Call, InsertBefore, nullptr, &DT, &LI);
        }))
      continue;
    if (Value *Leaf = ReloadedAtUsePoint.lookup(InsertBefore)) {
      U->set(Leaf);
      continue;
    }
    IRBuilder<> Builder(InsertBefore);
    Builder.SetCurrentDebugLocation(InsertBefore->getDebugLoc());
    Value *Reloaded = Builder.CreateAlignedLoad(
        AggregateType, &Home, Home.getAlign(), Home.getName() + ".reload");
    Value *Leaf = Builder.CreateExtractValue(
        Reloaded, Indices,
        Projection.hasName() ? Projection.getName() + ".reload"
                             : "argument.field.reload");
    for (auto [Template, OperandNo] : Recipe) {
      Instruction *Rematerialized = Template->clone();
      Rematerialized->setOperand(OperandNo, Leaf);
      Rematerialized->setName(Template->hasName()
                                  ? Template->getName() + ".reload"
                                  : "argument.address.reload");
      Rematerialized->setDebugLoc(InsertBefore->getDebugLoc());
      Rematerialized->insertBefore(InsertBefore->getIterator());
      Leaf = Rematerialized;
    }
    ReloadedAtUsePoint[InsertBefore] = Leaf;
    U->set(Leaf);
  }
  return Error::success();
}

bool aggregateArgumentFeedsAddressProjection(
    Value &V, SmallPtrSetImpl<Value *> &Visited) {
  if (!Visited.insert(&V).second)
    return false;
  for (User *U : V.users()) {
    if (isa<GetElementPtrInst>(U))
      return true;
    if (auto *Extract = dyn_cast<ExtractValueInst>(U);
        Extract && aggregateArgumentFeedsAddressProjection(*Extract, Visited))
      return true;
  }
  return false;
}

bool isHomeableBasePointer(Value &V, SmallPtrSetImpl<Value *> &Active,
                           DenseMap<Value *, bool> &Memo) {
  if (auto It = Memo.find(&V); It != Memo.end())
    return It->second;
  // Reject unanchored forwarding cycles. A PHI/select may choose different
  // object identities: the preheader store records the value selected on the
  // current path. It is homeable once every incoming chain reaches a concrete
  // base producer and the merge is available before the chosen loop.
  if (!Active.insert(&V).second)
    return false;

  bool IsBase = false;
  if (isa<ConstantPointerNull, Argument, LoadInst, CallBase, ExtractValueInst>(
          &V)) {
    IsBase = true;
  } else if (auto *Freeze = dyn_cast<FreezeInst>(&V)) {
    IsBase = isHomeableBasePointer(*Freeze->getOperand(0), Active, Memo);
  } else if (auto *Cast = dyn_cast<CastInst>(&V);
             Cast && Cast->getSrcTy()->isPointerTy() &&
             Cast->getDestTy()->isPointerTy() &&
             Cast->isNoopCast(Cast->getDataLayout())) {
    IsBase = isHomeableBasePointer(*Cast->getOperand(0), Active, Memo);
  } else if (auto *Phi = dyn_cast<PHINode>(&V)) {
    IsBase = llvm::all_of(Phi->incoming_values(), [&](Value *Incoming) {
      return isHomeableBasePointer(*Incoming, Active, Memo);
    });
  } else if (auto *Select = dyn_cast<SelectInst>(&V)) {
    IsBase = isHomeableBasePointer(*Select->getTrueValue(), Active, Memo) &&
             isHomeableBasePointer(*Select->getFalseValue(), Active, Memo);
  }

  Active.erase(&V);
  Memo[&V] = IsBase;
  return IsBase;
}

SmallVector<Loop *, 2>
findRepeatedPassiveLoopSafepoints(const StatepointPreservationPlan &Plan,
                                  LoopInfo &LI, DominatorTree &DT) {
  // A scalar home costs one function-wide frame object and one preheader
  // store. Require at least three safepoints and require the value to be
  // passive at one of them. A pointer may still be an argument of another
  // call in the cluster: reloading it at that use is precisely what ends its
  // cross-call SSA range. Direct and indirect calls have the same GC
  // preservation requirement here. Two-call loops remain below the
  // amortization boundary.
  constexpr unsigned MinLoopCallsToHome = 3;
  SmallVector<Loop *, 4> Candidates;
  for (Loop *TopLevel : LI) {
    SmallVector<Loop *, 4> Worklist{TopLevel};
    while (!Worklist.empty()) {
      Loop *Current = Worklist.pop_back_val();
      BasicBlock *Entry = Current->getLoopPreheader();
      if (!Entry)
        Entry = Current->getLoopPredecessor();
      BasicBlock *Header = Current->getHeader();
      bool HasUniqueEntryEdge =
          Entry && llvm::count_if(successors(Entry), [&](BasicBlock *Succ) {
                     return Succ == Header;
                   }) == 1;
      auto *Def = dyn_cast<Instruction>(Plan.V);
      bool AvailableOnEntryEdge =
          HasUniqueEntryEdge &&
          (!Def || (!Current->contains(Def->getParent()) &&
                    DT.dominates(Def, Entry->getTerminator())));
      if (AvailableOnEntryEdge) {
        unsigned LoopCalls =
            llvm::count_if(Plan.LiveCalls, [&](CallInst *Call) {
              return Current->contains(Call->getParent());
            });
        unsigned PassiveCalls =
            llvm::count_if(Plan.LiveCalls, [&](CallInst *Call) {
              return Current->contains(Call->getParent()) &&
                     !llvm::is_contained(Call->args(), Plan.V);
            });
        if (LoopCalls >= MinLoopCallsToHome && PassiveCalls != 0)
          Candidates.push_back(Current);
      }
      Worklist.append(Current->begin(), Current->end());
    }
  }

  // One SSA value can feed repeated-call loops on disjoint control-flow
  // paths. Picking only the statically largest sibling leaves the other path
  // on the relocate path and can miss the hot path entirely. Retain every
  // independently reachable candidate. When one candidate's initialization
  // dominates another, the first home is already authoritative for the
  // second region, so keep only the dominating candidate. This also chooses
  // an enclosing loop instead of reinitializing the home in an inner loop.
  SmallVector<Loop *, 2> Result;
  for (Loop *Candidate : Candidates) {
    BasicBlock *CandidateEntry = Candidate->getLoopPreheader();
    if (!CandidateEntry)
      CandidateEntry = Candidate->getLoopPredecessor();
    BasicBlockEdge CandidateEdge(CandidateEntry, Candidate->getHeader());
    bool Covered = llvm::any_of(Candidates, [&](Loop *Other) {
      if (Other == Candidate)
        return false;
      BasicBlock *OtherEntry = Other->getLoopPreheader();
      if (!OtherEntry)
        OtherEntry = Other->getLoopPredecessor();
      return OtherEntry && CandidateEntry &&
             DT.dominates(BasicBlockEdge(OtherEntry, Other->getHeader()),
                          CandidateEdge);
    });
    if (!Covered)
      Result.push_back(Candidate);
  }
  llvm::stable_sort(Result, [&](Loop *Left, Loop *Right) {
    auto FirstLiveCall = [&](Loop *L) -> size_t {
      for (auto [Index, Call] : llvm::enumerate(Plan.LiveCalls))
        if (L->contains(Call->getParent()))
          return Index;
      return std::numeric_limits<size_t>::max();
    };
    return FirstLiveCall(Left) < FirstLiveCall(Right);
  });
  return Result;
}

std::optional<BasicBlockEdge> loopHomeEntryEdge(Loop *HomeLoop) {
  if (!HomeLoop)
    return std::nullopt;
  BasicBlock *Entry = HomeLoop->getLoopPreheader();
  if (!Entry)
    Entry = HomeLoop->getLoopPredecessor();
  BasicBlock *Header = HomeLoop->getHeader();
  if (!Entry || llvm::count_if(successors(Entry), [&](BasicBlock *Succ) {
                  return Succ == Header;
                }) != 1)
    return std::nullopt;
  return BasicBlockEdge(Entry, Header);
}

CallInst *previousStatepointCall(Instruction &InsertBefore) {
  for (Instruction *I = InsertBefore.getPrevNode(); I; I = I->getPrevNode()) {
    auto *Call = dyn_cast<CallInst>(I);
    if (Call && !isa<GCStatepointInst>(Call) &&
        !isTerminalTailTransfer(*Call) && !isLeafCall(*Call))
      return Call;
  }
  return nullptr;
}

std::optional<unsigned>
countPointerHomeReloadPoints(const StatepointPreservationPlan &Plan,
                             Loop *HomeLoop, DominatorTree &DT) {
  std::optional<BasicBlockEdge> EntryEdge = loopHomeEntryEdge(HomeLoop);
  if (!EntryEdge)
    return std::nullopt;

  // The home is authoritative after entering the loop. Count one volatile
  // reload for each basic-block segment delimited by an ordinary safepoint;
  // uses in the same segment can safely share that reload. A high-frequency
  // address used throughout a numeric loop remains a poor home candidate when
  // those uses span as many statepoint-free regions as the calls it covers.
  SmallSet<std::pair<BasicBlock *, CallInst *>, 16> ReloadRegions;
  for (Use &U : Plan.V->uses()) {
    if (!DT.dominates(*EntryEdge, U))
      continue;
    Instruction *InsertBefore = aggregateUseInsertionPoint(U);
    if (!InsertBefore)
      return std::nullopt;
    ReloadRegions.insert(
        {InsertBefore->getParent(), previousStatepointCall(*InsertBefore)});
  }
  return ReloadRegions.size();
}

void retainProfitablePointerHomes(
    MapVector<Value *, StatepointPreservationPlan> &Plans, DominatorTree &DT) {
  // Decide independently for each pointer. Requiring a particular number of
  // sibling homes makes profitability depend on aggregate shape rather than
  // on the work this home removes. Reject only candidates whose dominated
  // material uses would add at least as many reload points as the live calls
  // covered by the home.
  for (auto &[V, Plan] : Plans) {
    if (Plan.Strategy != StatepointPreservationStrategy::TrackInHome ||
        !V->getType()->isPointerTy() || Plan.HomeLoops.empty())
      continue;

    Value *Candidate = V;
    StatepointPreservationPlan *PlanPtr = &Plan;
    llvm::erase_if(
        Plan.HomeLoops, [Candidate, PlanPtr, &Plans, &DT](Loop *HomeLoop) {
          std::optional<BasicBlockEdge> EntryEdge = loopHomeEntryEdge(HomeLoop);
          if (!EntryEdge)
            return true;
          SmallPtrSet<CallInst *, 16> DominatedLiveCalls;
          for (CallInst *Call : PlanPtr->LiveCalls)
            if (DT.dominates(*EntryEdge, Call->getParent()))
              DominatedLiveCalls.insert(Call);

          // A derived address which remains live through a call is rebuilt from
          // its base after the statepoint. Homing that base would therefore
          // leave both the home and its reloaded base in gc-live at the same
          // call, so it eliminates no relocate there. Charge those calls
          // against the benefit instead of selecting a nominally profitable but
          // duplicate root.
          SmallPtrSet<CallInst *, 16> ReintroducedBaseCalls;
          for (auto &[OtherV, OtherPlan] : Plans) {
            if (rematerializableDerivedBase(OtherV) != Candidate)
              continue;
            for (CallInst *Call : OtherPlan.LiveCalls)
              if (DominatedLiveCalls.contains(Call))
                ReintroducedBaseCalls.insert(Call);
          }
          unsigned EliminatedRelocates =
              DominatedLiveCalls.size() - ReintroducedBaseCalls.size();
          std::optional<unsigned> ReloadPoints =
              countPointerHomeReloadPoints(*PlanPtr, HomeLoop, DT);
          return !ReloadPoints || *ReloadPoints >= EliminatedRelocates;
        });
    if (Plan.HomeLoops.empty()) {
      Plan.Strategy = StatepointPreservationStrategy::RelocateSSA;
      continue;
    }
  }
}

bool hasMaterialPointerUse(Value &V, SmallPtrSetImpl<Value *> &Visited) {
  if (!Visited.insert(&V).second)
    return false;
  for (User *User : V.users()) {
    if (isa<GetElementPtrInst, LoadInst, StoreInst, AtomicRMWInst,
            AtomicCmpXchgInst, CallBase, ReturnInst, PtrToIntInst>(User))
      return true;
    if (isa<PHINode, SelectInst, FreezeInst, CastInst>(User) &&
        hasMaterialPointerUse(*cast<Value>(User), Visited))
      return true;
  }
  return false;
}

StatepointPreservationStrategy
chooseStatepointPreservationStrategy(StatepointPreservationPlan &Plan,
                                     Function &F, LoopInfo &LI,
                                     DominatorTree &DT) {
  Value *V = Plan.V;
  if (isFixedFrameAddress(V) || rematerializableDerivedBase(V))
    return StatepointPreservationStrategy::Rematerialize;

  if (!isGoCallingConv(F.getCallingConv()))
    return StatepointPreservationStrategy::RelocateSSA;

  Type *Ty = V->getType();
  if (Ty->isPointerTy()) {
    SmallPtrSet<Value *, 16> Active;
    DenseMap<Value *, bool> Memo;
    // Keep this deliberately narrower than "live across calls in a loop".
    // A home is useful when one pointer is repeatedly preserved for later use
    // rather than consumed by every call, independent of how those calls find
    // their callees.
    SmallPtrSet<Value *, 16> UseVisited;
    Plan.HomeLoops = findRepeatedPassiveLoopSafepoints(Plan, LI, DT);
    return !Plan.HomeLoops.empty() && !isa<Constant>(V) &&
                   isHomeableBasePointer(*V, Active, Memo) &&
                   hasMaterialPointerUse(*V, UseVisited)
               ? StatepointPreservationStrategy::TrackInHome
               : StatepointPreservationStrategy::RelocateSSA;
  }

  auto *Arg = dyn_cast<Argument>(V);
  if (!Arg || Arg->hasPassPointeeByValueCopyAttr())
    return StatepointPreservationStrategy::RelocateSSA;

  // A fixed ABI argument home avoids a new local frame object, but populating
  // it and reloading from it still has a cost. Amortize that cost only across
  // multiple static safepoints or a safepoint in a loop. LoopInfo is a static
  // profitability hint; correctness does not depend on recognizing a loop.
  bool AmortizesHome = Plan.LiveCalls.size() >= 2 ||
                       llvm::any_of(Plan.LiveCalls, [&](CallInst *Call) {
                         return LI.getLoopFor(Call->getParent()) != nullptr;
                       });
  if (!AmortizesHome || goabi::getPaddingPieces(Arg->getType()).any())
    return StatepointPreservationStrategy::RelocateSSA;

  if (!Ty->isAggregateType() || !containsPointer(Ty))
    return StatepointPreservationStrategy::RelocateSSA;
  SmallPtrSet<Value *, 8> Visited;
  return aggregateArgumentFeedsAddressProjection(*Arg, Visited)
             ? StatepointPreservationStrategy::TrackInHome
             : StatepointPreservationStrategy::RelocateSSA;
}

MapVector<Value *, StatepointPreservationPlan>
buildStatepointPreservationPlans(Function &F, LoopInfo &LI, DominatorTree &DT) {
  LivenessData StatepointLiveness = computeStatepointLiveness(F);
  MapVector<Value *, StatepointPreservationPlan> Plans;
  for (Instruction &I : instructions(F)) {
    auto *Call = dyn_cast<CallInst>(&I);
    if (!Call || isa<GCStatepointInst>(Call) || isLeafCall(*Call) ||
        isTerminalTailTransfer(*Call))
      continue;
    for (Value *Live : liveAtCall(*Call, StatepointLiveness)) {
      auto It = Plans.find(Live);
      if (It == Plans.end()) {
        Plans.insert({Live, {Live}});
        It = Plans.find(Live);
      }
      if (!llvm::is_contained(It->second.LiveCalls, Call))
        It->second.LiveCalls.push_back(Call);
    }
  }
  for (auto &[V, Plan] : Plans) {
    (void)V;
    Plan.Strategy = chooseStatepointPreservationStrategy(Plan, F, LI, DT);
  }
  return Plans;
}

SmallVector<WeakTrackingVH, 16> collectRelocatablePointerAggregates(
    const MapVector<Value *, StatepointPreservationPlan> &Plans) {
  SmallVector<WeakTrackingVH, 16> Candidates;
  for (const auto &[V, Plan] : Plans)
    if (Plan.Strategy == StatepointPreservationStrategy::RelocateSSA &&
        isPointerAggregateValue(V))
      Candidates.push_back(V);
  return Candidates;
}

Error homeStatepointArgument(Argument &Arg, ArrayRef<CallInst *> LiveCalls,
                             Function &F, DominatorTree &DT, LoopInfo &LI) {
  Type *ArgType = Arg.getType();
  BasicBlock &Entry = F.getEntryBlock();
  IRBuilder<> EntryBuilder(&*Entry.getFirstInsertionPt());
  auto *Home = EntryBuilder.CreateAlloca(
      ArgType, nullptr,
      Arg.hasName() ? Arg.getName() + ".statepoint.home"
                    : "argument.statepoint.home");
  Home->setAlignment(F.getDataLayout().getABITypeAlign(ArgType));
  StoreInst *Initialize =
      EntryBuilder.CreateAlignedStore(&Arg, Home, Home->getAlign());

  SmallVector<Use *, 16> OriginalUses;
  for (Use &U : Arg.uses())
    if (U.getUser() != Initialize)
      OriginalUses.push_back(&U);
  DenseMap<Instruction *, Value *> ReloadedAtUsePoint;

  for (Use *U : OriginalUses) {
    if (ArgType->isAggregateType()) {
      if (auto *Extract = dyn_cast<ExtractValueInst>(U->getUser())) {
        SmallVector<unsigned, 4> Indices(Extract->idx_begin(),
                                         Extract->idx_end());
        SmallVector<std::pair<Instruction *, unsigned>, 4> Recipe;
        if (Error Err = reloadAggregateArgumentProjection(
                *Home, ArgType, Indices, *Extract, Recipe, LiveCalls, DT, LI))
          return Err;
        if (Extract->use_empty())
          Extract->eraseFromParent();
        continue;
      }
    }
    Instruction *InsertBefore = aggregateUseInsertionPoint(*U);
    if (!InsertBefore)
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints cannot reload a non-instruction argument use");
    if (llvm::none_of(LiveCalls, [&](CallInst *Call) {
          return isPotentiallyReachable(Call, InsertBefore, nullptr, &DT, &LI);
        }))
      continue;
    if (Value *Reloaded = ReloadedAtUsePoint.lookup(InsertBefore)) {
      U->set(Reloaded);
      continue;
    }
    IRBuilder<> Builder(InsertBefore);
    Builder.SetCurrentDebugLocation(InsertBefore->getDebugLoc());
    Value *Reloaded = Builder.CreateAlignedLoad(ArgType, Home, Home->getAlign(),
                                                Home->getName() + ".reload");
    ReloadedAtUsePoint[InsertBefore] = Reloaded;
    U->set(Reloaded);
  }
  return Error::success();
}

Error homeStatepointPointer(Value &V, Loop &HomeLoop, Function &F,
                            DominatorTree &DT, LoopInfo &LI) {
  assert(V.getType()->isPointerTy() && "expected scalar pointer home");
  BasicBlock *Preheader = HomeLoop.getLoopPreheader();
  if (!Preheader) {
    BasicBlock *Predecessor = HomeLoop.getLoopPredecessor();
    if (!Predecessor)
      return Error::success();
    Preheader =
        SplitEdge(Predecessor, HomeLoop.getHeader(), &DT, &LI, nullptr,
                  HomeLoop.getHeader()->getName() + ".statepoint.preheader");
    if (!Preheader)
      return createStringError(
          std::errc::not_supported,
          "GoALLC cannot split a selected loop home entry edge");
  }

  BasicBlock &Entry = F.getEntryBlock();
  IRBuilder<> EntryBuilder(&*Entry.getFirstInsertionPt());
  StringRef Name = V.hasName() ? V.getName() : "pointer";
  auto *Home = EntryBuilder.CreateAlloca(V.getType(), nullptr,
                                         Name + ".statepoint.home");
  Home->setAlignment(F.getDataLayout().getABITypeAlign(V.getType()));

  Instruction *InitializeBefore = Preheader->getTerminator();
  IRBuilder<> InitializeBuilder(InitializeBefore);
  InitializeBuilder.SetCurrentDebugLocation(InitializeBefore->getDebugLoc());
  CallInst *LifetimeStart = InitializeBuilder.CreateLifetimeStart(Home);
  StoreInst *Initialize =
      InitializeBuilder.CreateAlignedStore(&V, Home, Home->getAlign());

  SmallVector<Use *, 16> OriginalUses;
  for (Use &U : V.uses())
    if (U.getUser() != Initialize)
      OriginalUses.push_back(&U);
  struct ReloadRegion {
    BasicBlock *Block;
    CallInst *PreviousCall;
    Instruction *InsertBefore;
    SmallVector<Use *, 4> Uses;
  };
  SmallVector<ReloadRegion, 8> ReloadRegions;
  for (Use *U : OriginalUses) {
    if (!DT.dominates(Initialize, *U))
      continue;
    Instruction *InsertBefore = aggregateUseInsertionPoint(*U);
    if (!InsertBefore)
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints cannot reload a non-instruction pointer use");
    CallInst *PreviousCall = previousStatepointCall(*InsertBefore);
    auto It = llvm::find_if(ReloadRegions, [&](const ReloadRegion &Region) {
      return Region.Block == InsertBefore->getParent() &&
             Region.PreviousCall == PreviousCall;
    });
    if (It == ReloadRegions.end()) {
      ReloadRegions.push_back(
          {InsertBefore->getParent(), PreviousCall, InsertBefore, {U}});
      continue;
    }
    if (InsertBefore->comesBefore(It->InsertBefore))
      It->InsertBefore = InsertBefore;
    It->Uses.push_back(U);
  }
  if (ReloadRegions.empty()) {
    Initialize->eraseFromParent();
    LifetimeStart->eraseFromParent();
    Home->eraseFromParent();
    return Error::success();
  }

  for (ReloadRegion &Region : ReloadRegions) {
    IRBuilder<> Builder(Region.InsertBefore);
    Builder.SetCurrentDebugLocation(Region.InsertBefore->getDebugLoc());
    auto *Reload = Builder.CreateAlignedLoad(
        V.getType(), Home, Home->getAlign(), Name + ".statepoint.reload");
    // The collector may rewrite Home while executing any intervening
    // statepoint. The gc-live operand describes the stack map, but it is not
    // an LLVM memory dependence: ordinary alias analysis knows that the local
    // alloca is not passed to the callee and may otherwise hoist this reload
    // above the call. Keep one volatile reload in each statepoint-free region
    // so CodeGen cannot resurrect the pre-relocation address.
    Reload->setVolatile(true);
    for (Use *U : Region.Uses)
      U->set(Reload);
  }
  return Error::success();
}

// Build one preservation plan for every live pointer or pointer-bearing SSA
// carrier, then materialize only the homes selected by the profitability
// policy. Scalar relocate and aggregate-leaf scalarization remain the default;
// in particular this never makes a whole aggregate a post-statepoint SSA
// definition and therefore cannot introduce relocate-driven aggregate PHIs.
Error applyStatepointPreservationPlans(
    MapVector<Value *, StatepointPreservationPlan> &Plans, Function &F,
    DominatorTree &DT, LoopInfo &LI) {
  struct PendingPreservation {
    WeakTrackingVH V;
    SmallVector<CallInst *, 4> LiveCalls;
    SmallVector<Loop *, 2> HomeLoops;
  };
  SmallVector<PendingPreservation, 8> Pending;
  for (auto &[V, Plan] : Plans)
    if (Plan.Strategy == StatepointPreservationStrategy::TrackInHome)
      Pending.push_back({V, Plan.LiveCalls, Plan.HomeLoops});

  // Aggregate homes may erase pointer projections which were independently
  // selected for a scalar home. Apply them first and let weak handles discard
  // the now-redundant decisions.
  llvm::stable_sort(Pending, [](const PendingPreservation &Left,
                                const PendingPreservation &Right) {
    Value *LeftValue = Left.V;
    Value *RightValue = Right.V;
    return LeftValue && RightValue && !LeftValue->getType()->isPointerTy() &&
           RightValue->getType()->isPointerTy();
  });
  for (PendingPreservation &P : Pending) {
    Value *V = P.V;
    if (!V)
      continue;
    if (!V->getType()->isPointerTy()) {
      auto *Arg = cast<Argument>(V);
      if (Error Err = homeStatepointArgument(*Arg, P.LiveCalls, F, DT, LI))
        return Err;
      continue;
    }
    for (Loop *HomeLoop : P.HomeLoops)
      if (Error Err = homeStatepointPointer(*V, *HomeLoop, F, DT, LI))
        return Err;
  }
  return Error::success();
}

Error scalarizeLivePointerAggregates(
    Function &F, ArrayRef<WeakTrackingVH> CandidateHandles,
    MapVector<Value *, Value *> &DirectPointerLeafSources) {
  ValueSet Candidates;
  for (const WeakTrackingVH &Handle : CandidateHandles)
    if (Value *Candidate = Handle)
      Candidates.insert(Candidate);

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
        extractAggregateLeaves(*Candidate, Leaves, F, DirectPointerLeafSources);
    if (!LeafValuesOrErr)
      return LeafValuesOrErr.takeError();
    SmallVector<Value *, 8> LeafValues = std::move(*LeafValuesOrErr);
    DenseMap<Instruction *, Value *> RebuiltAtUsePoint;

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
      if (Value *Rebuilt = RebuiltAtUsePoint.lookup(InsertBefore)) {
        U->set(Rebuilt);
        continue;
      }
      IRBuilder<> Builder(InsertBefore);
      Builder.SetCurrentDebugLocation(InsertBefore->getDebugLoc());
      Value *Rebuilt = PoisonValue::get(Candidate->getType());
      for (auto [Leaf, LeafValue] : llvm::zip(Leaves, LeafValues)) {
        Rebuilt = Builder.CreateInsertValue(
            Rebuilt, LeafValue, Leaf.Indices,
            Candidate->hasName() ? Candidate->getName() + ".rebuilt" : "");
      }
      RebuiltAtUsePoint[InsertBefore] = Rebuilt;
      U->set(Rebuilt);
    }
    for (Value *LeafValue : LeafValues)
      if (auto *LeafInst = dyn_cast<Instruction>(LeafValue);
          LeafInst && LeafInst->use_empty()) {
        DirectPointerLeafSources.erase(LeafInst);
        LeafInst->eraseFromParent();
      }
  }
  return Error::success();
}

DirectPointerLeafAliasGroups buildDirectPointerLeafAliasGroups(
    const MapVector<Value *, Value *> &DirectPointerLeafSources) {
  DirectPointerLeafAliasGroups AliasGroups;
  for (auto [Leaf, DirectSource] : DirectPointerLeafSources) {
    // Scalarization can be nested. Follow exact insertvalue provenance to its
    // original scalar root and group the complete alias chain. Choosing one
    // canonical root per group avoids making an earlier coalescing decision
    // refer to a root removed by a later nested leaf.
    SmallVector<Value *, 4> Aliases{Leaf};
    Value *Root = DirectSource;
    while (Value *Source = DirectPointerLeafSources.lookup(Root)) {
      Aliases.push_back(Root);
      Root = Source;
    }
    Aliases.push_back(Root);

    SmallVector<Value *, 4> &Group = AliasGroups[Root];
    for (Value *Alias : Aliases)
      if (!llvm::is_contained(Group, Alias))
        Group.push_back(Alias);
  }
  return AliasGroups;
}

std::optional<ValueSet> collectCallArgumentComponents(CallInst &Call) {
  ValueSet Components;
  for (Value *Argument : Call.args()) {
    Components.insert(Argument);
    if (!Argument->getType()->isAggregateType())
      continue;

    SmallVector<AggregateLeaf, 8> Leaves;
    SmallVector<unsigned, 4> Path;
    if (Error Err =
            enumerateAggregateLeaves(Argument->getType(), Path, Leaves)) {
      consumeError(std::move(Err));
      return std::nullopt;
    }
    for (const AggregateLeaf &Leaf : Leaves)
      if (Value *Component = FindInsertedValue(Argument, Leaf.Indices))
        Components.insert(Component);
  }
  return Components;
}

void coalesceDirectAggregateLeafRoots(
    SafepointRecord &Record, const DirectPointerLeafAliasGroups &AliasGroups) {
  std::optional<ValueSet> CallArgumentComponents =
      collectCallArgumentComponents(*Record.Call);
  // If an aggregate call argument cannot be enumerated, preserve every alias
  // group just as the per-candidate analysis did. Coalescing must remain
  // fail-closed at ABI carrier boundaries.
  if (!CallArgumentComponents)
    return;

  for (const auto &[Root, Aliases] : AliasGroups) {
    if (!Record.Live.contains(Root))
      continue;

    // Go calling conventions can assign equivalent argument and live-through
    // identities different machine locations. Preserve the whole group if
    // the original source supplies an argument or multiple aliases do. When
    // exactly one scalarized leaf supplies an argument, keep that leaf as the
    // shared root so both roles consume the same relocated identity.
    Value *CallArgumentLeaf = nullptr;
    bool PreserveGroup = false;
    for (Value *Alias : Aliases) {
      if (!CallArgumentComponents->contains(Alias))
        continue;
      if (Alias == Root || CallArgumentLeaf) {
        PreserveGroup = true;
        break;
      }
      CallArgumentLeaf = Alias;
    }
    if (PreserveGroup)
      continue;

    // A scalarized call-argument leaf can replace only the canonical identity
    // of a direct pointer result. Aggregate extracts, loads, and other carrier
    // identities can acquire distinct ABI locations even when their IR values
    // are equal at the call boundary.
    if (CallArgumentLeaf && !isa<CallInst>(Root))
      continue;

    Value *Canonical = CallArgumentLeaf ? CallArgumentLeaf : Root;
    if (!Record.Live.contains(Canonical))
      continue;
    for (Value *Alias : Aliases) {
      if (Alias == Canonical || !Record.Live.contains(Alias))
        continue;
      Record.Live.remove(Alias);
      Record.CoalescedLiveRoots.push_back({Alias, Canonical});
    }
  }
}

Error enumeratePointerFrameLeaves(Type *Ty, const DataLayout &DL,
                                  SmallVectorImpl<unsigned> &Path,
                                  uint64_t Offset,
                                  SmallVectorImpl<PointerFrameLeaf> &Leaves) {
  if (auto *ST = dyn_cast<StructType>(Ty)) {
    if (ST->isOpaque())
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints cannot describe an opaque alloca struct");
    const StructLayout *Layout = DL.getStructLayout(ST);
    for (auto [Index, ElementTy] : llvm::enumerate(ST->elements())) {
      TypeSize LayoutOffset = Layout->getElementOffset(Index);
      if (LayoutOffset.isScalable())
        return createStringError(
            std::errc::not_supported,
            "GoALLC statepoints do not support scalable alloca structs");
      auto ElementOffset =
          checkedAddUnsigned(Offset, LayoutOffset.getFixedValue());
      if (!ElementOffset)
        return createStringError(
            std::errc::value_too_large,
            "GoALLC statepoint alloca pointer offset overflow");
      Path.push_back(Index);
      if (Error Err = enumeratePointerFrameLeaves(ElementTy, DL, Path,
                                                  *ElementOffset, Leaves))
        return std::move(Err);
      Path.pop_back();
    }
    return Error::success();
  }
  if (auto *AT = dyn_cast<ArrayType>(Ty)) {
    if (AT->getNumElements() > std::numeric_limits<unsigned>::max())
      return createStringError(
          std::errc::value_too_large,
          "GoALLC statepoints cannot enumerate an oversized alloca array");
    TypeSize ElementSize = DL.getTypeAllocSize(AT->getElementType());
    if (ElementSize.isScalable())
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints do not support scalable alloca elements");
    for (uint64_t Index = 0; Index != AT->getNumElements(); ++Index) {
      auto RelativeOffset =
          checkedMulUnsigned(Index, ElementSize.getFixedValue());
      auto ElementOffset = RelativeOffset
                               ? checkedAddUnsigned(Offset, *RelativeOffset)
                               : std::optional<uint64_t>();
      if (!ElementOffset)
        return createStringError(
            std::errc::value_too_large,
            "GoALLC statepoint alloca pointer offset overflow");
      Path.push_back(static_cast<unsigned>(Index));
      if (Error Err = enumeratePointerFrameLeaves(AT->getElementType(), DL,
                                                  Path, *ElementOffset, Leaves))
        return std::move(Err);
      Path.pop_back();
    }
    return Error::success();
  }
  if (Ty->isVectorTy() && containsPointer(Ty))
    return createStringError(
        std::errc::not_supported,
        "GoALLC statepoints do not support pointer vectors in allocas");
  if (auto *PointerTy = dyn_cast<PointerType>(Ty)) {
    TypeSize PointerSize = DL.getTypeStoreSize(PointerTy);
    if (PointerTy->getAddressSpace() != 0 || PointerSize.isScalable() ||
        PointerSize.getFixedValue() != DL.getPointerSize(0))
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints require default-address-space pointer words in "
          "allocas");
    Leaves.push_back({SmallVector<unsigned, 4>(ArrayRef<unsigned>(Path)),
                      Offset, PointerTy});
  }
  return Error::success();
}

enum class PointerFrameKind { Alloca, FixedArgument };

Expected<PointerFrameLayout>
pointerFrameLayout(Type *StorageType, Align Alignment, const DataLayout &DL,
                   PointerFrameKind Kind, std::optional<Align> StackAlign) {
  bool IsAlloca = Kind == PointerFrameKind::Alloca;
  TypeSize AllocationSize = DL.getTypeAllocSize(StorageType);
  if (AllocationSize.isScalable())
    return createStringError(
        std::errc::not_supported,
        IsAlloca ? "GoALLC statepoints do not support scalable "
                   "pointer-containing allocas"
                 : "GoALLC statepoints do not support scalable fixed argument "
                   "layouts");
  if (IsAlloca && StackAlign && Alignment > *StackAlign)
    return createStringError(
        std::errc::not_supported,
        "GoALLC statepoints do not support realigned pointer-containing "
        "allocas");

  uint64_t ByteSize = AllocationSize.getFixedValue();
  uint64_t PointerSize = DL.getPointerSize(0);
  bool InvalidLayout = !PointerSize || !ByteSize ||
                       ByteSize % PointerSize != 0 ||
                       Alignment < DL.getABITypeAlign(StorageType);
  auto LayoutError = [&]() {
    return createStringError(
        std::errc::not_supported,
        IsAlloca
            ? "GoALLC statepoints require pointer-aligned fixed alloca layouts"
            : "GoALLC statepoints require pointer-aligned fixed argument "
              "layouts");
  };
  // Preserve the established diagnostics: fixed arguments validate their
  // outer layout first, while allocas enumerate unsupported leaf types first.
  if (!IsAlloca && InvalidLayout)
    return LayoutError();

  SmallVector<PointerFrameLeaf, 8> Leaves;
  SmallVector<unsigned, 4> Path;
  if (Error Err = enumeratePointerFrameLeaves(StorageType, DL, Path, 0, Leaves))
    return std::move(Err);
  if (IsAlloca && InvalidLayout)
    return LayoutError();

  uint64_t BitCount = ByteSize / PointerSize;
  SmallVector<uint64_t, 4> BitmapWords((BitCount + 63) / 64, 0);
  for (const PointerFrameLeaf &Leaf : Leaves) {
    if (Leaf.Offset % PointerSize != 0 || Leaf.Offset >= ByteSize)
      return createStringError(
          std::errc::not_supported,
          IsAlloca
              ? "GoALLC statepoint alloca pointer slot is not pointer-aligned"
              : "GoALLC statepoint fixed argument pointer slot is not "
                "pointer-aligned");
    uint64_t Bit = Leaf.Offset / PointerSize;
    uint64_t Mask = uint64_t(1) << (Bit % 64);
    if (BitmapWords[Bit / 64] & Mask)
      return createStringError(
          std::errc::invalid_argument,
          IsAlloca ? "GoALLC statepoint alloca pointer slots overlap"
                   : "GoALLC statepoint fixed argument pointer slots overlap");
    BitmapWords[Bit / 64] |= Mask;
  }

  return PointerFrameLayout{ByteSize, Alignment.value(), BitCount,
                            std::move(BitmapWords), std::move(Leaves)};
}

Value *pointerAllocaLeafAddress(IRBuilder<> &Builder, AllocaInst &Alloca,
                                const PointerFrameLeaf &Leaf,
                                const Twine &Name) {
  if (Leaf.Indices.empty())
    return &Alloca;
  SmallVector<Value *, 8> GEPIndices;
  GEPIndices.push_back(Builder.getInt32(0));
  for (unsigned Index : Leaf.Indices)
    GEPIndices.push_back(Builder.getInt32(Index));
  return Builder.CreateInBoundsGEP(Alloca.getAllocatedType(), &Alloca,
                                   GEPIndices, Name);
}

std::string allocaLeafName(AllocaInst &Alloca, const PointerFrameLeaf &Leaf) {
  std::string Name =
      Alloca.hasName() ? (Alloca.getName() + ".gc.leaf").str() : "gc.leaf";
  for (unsigned Index : Leaf.Indices)
    Name += "." + std::to_string(Index);
  return Name;
}

Expected<bool> deferResultAlloca(AllocaInst &Alloca) {
  MDNode *MD = Alloca.getMetadata(GoDeferResultMD);
  if (!MD)
    return false;
  if (MD->getNumOperands() != 0)
    return createStringError(std::errc::invalid_argument,
                             "GoALLC defer-result metadata must be empty");
  return true;
}

Expected<std::optional<OpenDeferInfo>> collectOpenDeferInfo(Function &F) {
  const DataLayout &DL = F.getDataLayout();
  OpenDeferInfo Info;

  for (Instruction &I : instructions(F)) {
    auto *Alloca = dyn_cast<AllocaInst>(&I);
    if (!Alloca)
      continue;
    MDNode *BitsMD = Alloca->getMetadata(GoOpenDeferBitsMD);
    MDNode *SlotsMD = Alloca->getMetadata(GoOpenDeferSlotsMD);
    if (!BitsMD && !SlotsMD)
      continue;
    if (!Alloca->isStaticAlloca() || Alloca->getParent() != &F.getEntryBlock())
      return createStringError(
          std::errc::not_supported,
          "GoALLC open-defer state requires fixed entry-block allocas");
    if (BitsMD) {
      if (BitsMD->getNumOperands() != 0 || Info.Bits || SlotsMD ||
          !Alloca->getAllocatedType()->isIntegerTy(8))
        return createStringError(std::errc::invalid_argument,
                                 "GoALLC open-defer bits metadata is invalid");
      Info.Bits = Alloca;
    } else {
      auto *SlotsType = dyn_cast<ArrayType>(Alloca->getAllocatedType());
      auto *CountMD = SlotsMD && SlotsMD->getNumOperands() == 1
                          ? dyn_cast<ConstantAsMetadata>(SlotsMD->getOperand(0))
                          : nullptr;
      auto *Count =
          CountMD ? dyn_cast<ConstantInt>(CountMD->getValue()) : nullptr;
      if (Info.Slots || !SlotsType ||
          !SlotsType->getElementType()->isPointerTy() || !Count ||
          Count->getValue().getActiveBits() > 64 || Count->isZero() ||
          Count->getZExtValue() != SlotsType->getNumElements() ||
          Alloca->getAlign() < DL.getPointerABIAlignment(0))
        return createStringError(
            std::errc::invalid_argument,
            "GoALLC open-defer slots must be one aligned pointer array");
      Info.Slots = Alloca;
      Info.SlotCount = Count->getZExtValue();
    }
  }

  if (!Info.Bits && !Info.Slots)
    return std::optional<OpenDeferInfo>();
  if (!Info.Bits || !Info.Slots) {
    // The frontend emits both objects, but optimization can delete every defer
    // registration path and scalarize the remaining zero-only slots alloca.
    // The replacement alloca does not inherit custom metadata. With no complete
    // optimized frame state left to encode, discard the surviving marker too.
    if (Info.Bits)
      Info.Bits->setMetadata(GoOpenDeferBitsMD, nullptr);
    if (Info.Slots)
      Info.Slots->setMetadata(GoOpenDeferSlotsMD, nullptr);
    return std::optional<OpenDeferInfo>();
  }

  Info.Bits->setMetadata(GoOpenDeferBitsMD, nullptr);
  Info.Slots->setMetadata(GoOpenDeferSlotsMD, nullptr);
  return std::optional<OpenDeferInfo>(std::move(Info));
}

// Return true when the optimized IR can make the address observable outside
// compiler-controlled direct accesses. This is deliberately a structural
// post-optimization decision: the frontend Addrtaken bit is provenance, not a
// promise that prevents SROA or forces a surviving frame address to remain a
// stack object.
bool isWholeAllocaGoRetUse(const AllocaInst &Alloca, Value *Address,
                           const Use &U) {
  auto *Call = dyn_cast<CallInst>(U.getUser());
  if (!Call || !Call->isArgOperand(&U) ||
      !isGoCallingConv(Call->getCallingConv()))
    return false;
  unsigned ArgNo = Call->getArgOperandNo(&U);
  if (!Call->paramHasAttr(ArgNo, Attribute::GoRet) ||
      Call->getParamGoRetType(ArgNo) != Alloca.getAllocatedType())
    return false;

  int64_t Offset = 0;
  const DataLayout &DL = Call->getDataLayout();
  if (GetPointerBaseWithConstantOffset(Address, Offset, DL) != &Alloca ||
      Offset != 0)
    return false;
  std::optional<Align> ParamAlign = Call->getParamAlign(ArgNo);
  return ParamAlign && Alloca.getAlign() >= *ParamAlign;
}

bool addressNeedsStackObject(Value &Base) {
  SmallVector<Value *, 16> Worklist{&Base};
  SmallPtrSet<Value *, 16> Seen;
  while (!Worklist.empty()) {
    Value *Address = Worklist.pop_back_val();
    if (!Seen.insert(Address).second)
      continue;
    for (Use &U : Address->uses()) {
      auto *I = dyn_cast<Instruction>(U.getUser());
      if (!I)
        return true;

      switch (classifyFrameAddressUse(U)) {
      case FrameAddressUseKind::Derivation:
        if (!I->getType()->isPointerTy())
          return true;
        Worklist.push_back(I);
        continue;
      case FrameAddressUseKind::TerminalMemory:
        if (auto *Load = dyn_cast<LoadInst>(I)) {
          if (Load->isAtomic() || Load->isVolatile())
            return true;
          continue;
        }
        if (auto *Store = dyn_cast<StoreInst>(I)) {
          if (Store->isAtomic() || Store->isVolatile())
            return true;
          continue;
        }
        if (isa<MemIntrinsic>(I))
          continue;
        // Atomic read-modify-write operations expose the frame object.
        return true;
      case FrameAddressUseKind::LifetimeOrDebug:
      case FrameAddressUseKind::FakeUse:
        continue;
      case FrameAddressUseKind::FirstClass:
        if (isa<PHINode, SelectInst, FreezeInst>(I) &&
            I->getType()->isPointerTy() &&
            fixedFrameProvenanceBase(I) == &Base) {
          Worklist.push_back(I);
          continue;
        }
        if (isa<ICmpInst>(I))
          continue;
        // A typed goret operand denotes the callee-written ABI result area;
        // the callee does not receive or observe this IR carrier address.
        // Keep exact whole-object carriers on the compiler-controlled fixed
        // frame path. Any other use of the same address is still examined
        // independently and can require a StackObject.
        if (auto *Alloca = dyn_cast<AllocaInst>(&Base);
            Alloca && isWholeAllocaGoRetUse(*Alloca, Address, U))
          continue;
        // A mixed-object merge, passing, storing, returning, converting,
        // inline asm, and every other escaping SSA use makes the address
        // observable outside compiler-controlled Base+Offset rematerialization.
        return true;
      }
      llvm_unreachable("unknown frame address use kind");
    }
  }
  return false;
}

Error collectPointerAllocaLifetimeMarkers(
    Function &F, SmallVectorImpl<PointerAllocaRecord> &PointerAllocas) {
  DenseMap<const AllocaInst *, PointerAllocaRecord *> Records;
  for (PointerAllocaRecord &Record : PointerAllocas)
    Records[Record.Alloca] = &Record;

  for (Instruction &I : instructions(F)) {
    auto *Lifetime = dyn_cast<LifetimeIntrinsic>(&I);
    if (!Lifetime)
      continue;
    Value *Pointer = Lifetime->getArgOperand(0);
    auto *Alloca = dyn_cast<AllocaInst>(Pointer);
    if (!Alloca) {
      if (const AllocaInst *Underlying =
              findAllocaForValue(Pointer, /*OffsetZero=*/true);
          Underlying && Records.contains(Underlying))
        return createStringError(
            std::errc::not_supported,
            "GoALLC statepoint lifetimes must reference the whole pointer "
            "alloca directly");
      continue;
    }
    auto It = Records.find(Alloca);
    if (It != Records.end())
      It->second->LifetimeMarkers.push_back(Lifetime);
  }
  return Error::success();
}

void collectPointerAllocaAddressUses(PointerAllocaRecord &Record,
                                     const DominatorTree &DT) {
  bool HasLifetimeStart =
      llvm::any_of(Record.LifetimeMarkers, [](IntrinsicInst *Marker) {
        return Marker->getIntrinsicID() == Intrinsic::lifetime_start;
      });
  SmallPtrSet<Instruction *, 16> SeenUses;
  SmallPtrSet<CallInst *, 4> CandidateGoRetDefs;
  SmallPtrSet<CallInst *, 4> NonGoRetCallUses;
  visitFixedFrameAddressUses(
      *Record.Alloca,
      [&](Value *Address, Use &U, Instruction *I, FrameAddressUseKind Kind) {
        if (!I) {
          Record.ActivityUnclear = true;
          return;
        }
        if (Kind == FrameAddressUseKind::Derivation) {
          if (!I->getType()->isPointerTy())
            Record.ActivityUnclear = true;
          return;
        }
        if (Kind == FrameAddressUseKind::LifetimeOrDebug ||
            (Kind == FrameAddressUseKind::FirstClass &&
             isa<PHINode, SelectInst, FreezeInst>(I)))
          return;
        if (auto *Call = dyn_cast<CallInst>(I)) {
          if (isWholeAllocaGoRetUse(*Record.Alloca, Address, U))
            CandidateGoRetDefs.insert(Call);
          else
            NonGoRetCallUses.insert(Call);
        }
        if (Kind == FrameAddressUseKind::FirstClass && HasLifetimeStart &&
            !llvm::any_of(Record.LifetimeMarkers, [&](IntrinsicInst *Marker) {
              return Marker->getIntrinsicID() == Intrinsic::lifetime_start &&
                     DT.dominates(Marker, U);
            })) {
          // LLVM may hoist a pure address operation outside the source VarDef
          // interval. The same operation generates content liveness, so retain
          // the old whole-lifetime fallback for the physical storage. The
          // original markers remain liveness kills until activity has been
          // computed.
          Record.WholeLifetime = true;
        }
        if (SeenUses.insert(I).second)
          Record.AddressUses.push_back(I);
      });
  for (CallInst *Call : CandidateGoRetDefs)
    if (!NonGoRetCallUses.contains(Call))
      Record.GoRetDefs.push_back(Call);
}

void transferPointerAllocaLiveness(const PointerAllocaRecord &Record,
                                   const Instruction &I, bool &Live) {
  if (llvm::is_contained(Record.LifetimeMarkers, &I) ||
      llvm::is_contained(Record.GoRetDefs, &I))
    Live = false;
  else if (llvm::is_contained(Record.AddressUses, &I))
    Live = true;
}

bool pointerAllocaLiveInBlock(const PointerAllocaRecord &Record,
                              const BasicBlock &BB, bool LiveOut) {
  bool Live = LiveOut;
  for (const Instruction &I : llvm::reverse(BB))
    transferPointerAllocaLiveness(Record, I, Live);
  return Live;
}

// Go VarDef is a complete initialization boundary. Treat its lifetime.start
// as a backward kill and terminal address operations as uses. This gives each
// start a path-sensitive interval whose end is the final real use, without
// inserting lifetime.end or changing the producer's initialization sequence.
Error computePointerAllocaActivity(
    Function &F, SmallVectorImpl<PointerAllocaRecord> &PointerAllocas,
    const SmallPtrSetImpl<const CallInst *> &SafepointCalls,
    const DominatorTree &DT) {
  for (PointerAllocaRecord &Record : PointerAllocas) {
    collectPointerAllocaAddressUses(Record, DT);
    if (Record.ActivityUnclear)
      return createStringError(
          std::errc::not_supported,
          "GoALLC cannot determine pointer alloca live-out activity");

    DenseMap<const BasicBlock *, bool> LiveIn;
    bool Changed;
    do {
      Changed = false;
      for (BasicBlock &BB : llvm::reverse(F)) {
        bool LiveOut = llvm::any_of(successors(&BB), [&](BasicBlock *Succ) {
          return LiveIn.lookup(Succ);
        });
        bool NewLiveIn = pointerAllocaLiveInBlock(Record, BB, LiveOut);
        if (NewLiveIn != LiveIn.lookup(&BB)) {
          LiveIn[&BB] = NewLiveIn;
          Changed = true;
        }
      }
    } while (Changed);

    for (BasicBlock &BB : F) {
      bool Live = llvm::any_of(successors(&BB), [&](BasicBlock *Succ) {
        return LiveIn.lookup(Succ);
      });
      for (Instruction &I : llvm::reverse(BB)) {
        // A caller stack map describes values live after the call. Apply the
        // current instruction's use transfer only after recording the
        // safepoint, so an address used solely as this call's argument is not
        // a caller gc-live root.
        auto *Call = dyn_cast<CallInst>(&I);
        bool IsGoRetDef = llvm::is_contained(Record.GoRetDefs, Call);
        if (Call && SafepointCalls.contains(Call) && !IsGoRetDef &&
            (Live || !DT.isReachableFromEntry(Call->getParent())))
          Record.ActiveCalls.push_back(Call);
        transferPointerAllocaLiveness(Record, I, Live);
      }
    }
  }
  return Error::success();
}

void protectStackObjectsFromColoring(
    MutableArrayRef<PointerAllocaRecord> PointerAllocas) {
  for (PointerAllocaRecord &Record : PointerAllocas) {
    if (!Record.NeedsStackObject && !Record.DeferResult &&
        !Record.OpenDeferSlot)
      continue;
    // A Go StackObject has function-wide identity and layout metadata. A defer
    // result likewise remains a root at every statepoint. Neither can share
    // storage with another lifetime-disjoint alloca.
    Record.Alloca->setMetadata(
        StackColoringNoMergeMD,
        MDNode::get(Record.Alloca->getContext(), ArrayRef<Metadata *>()));
  }
}

// Re-establish the Go zero-value invariant after the ordinary optimization
// pipeline. LLVM can remove source-level zero stores when no LLVM load
// observes them, but Go's stack scanner independently observes every pointer
// word selected by a callsite bitmap. Ensure each pointer-containing alloca is
// zero at the start of every source lifetime; marker-free allocas
// conservatively start at function entry. If ordinary stores already
// initialize every GC-visible pointer slot before the first safepoint, no
// additional zero is needed; non-pointer fields and padding are irrelevant.
// The frontend's volatile defer initialization remains authoritative.
// Fixed-size memset.inline cannot fall back to a hosted libcall during GoObj
// lowering.
void updateInitializedPointerSlots(SmallBitVector &Initialized,
                                   Value *Destination, uint64_t WriteSize,
                                   bool WritesZero,
                                   ArrayRef<uint64_t> StoredPointerOffsets,
                                   const PointerAllocaRecord &Record,
                                   const DataLayout &DL) {
  if (getUnderlyingObject(Destination) != Record.Alloca)
    return;

  int64_t SignedOffset = 0;
  Value *Base = GetPointerBaseWithConstantOffset(Destination, SignedOffset, DL);
  if (Base != Record.Alloca || SignedOffset < 0) {
    Initialized.reset();
    return;
  }
  uint64_t WriteBegin = static_cast<uint64_t>(SignedOffset);
  std::optional<uint64_t> WriteEnd = checkedAddUnsigned(WriteBegin, WriteSize);
  if (!WriteEnd) {
    Initialized.reset();
    return;
  }

  uint64_t PointerSize = DL.getPointerSize(0);
  for (auto [Index, Leaf] : llvm::enumerate(Record.Layout.Leaves)) {
    uint64_t SlotBegin = Leaf.Offset;
    uint64_t SlotEnd = SlotBegin + PointerSize;
    if (*WriteEnd <= SlotBegin || WriteBegin >= SlotEnd)
      continue;

    bool CoversSlot = WriteBegin <= SlotBegin && *WriteEnd >= SlotEnd;
    bool StoresPointer =
        CoversSlot &&
        llvm::is_contained(StoredPointerOffsets, SlotBegin - WriteBegin);
    Initialized[Index] = CoversSlot && (WritesZero || StoresPointer);
  }
}

bool hasInitializedPointerSlotsBeforeSafepoint(
    Instruction *Begin, const PointerAllocaRecord &Record,
    const DataLayout &DL) {
  SmallBitVector Initialized(Record.Layout.Leaves.size());
  if (Record.ActiveCalls.empty())
    return true;
  for (Instruction *I = Begin; I; I = I->getNextNode()) {
    if (auto *Store = dyn_cast<StoreInst>(I)) {
      Value *StoredValue = Store->getValueOperand();
      TypeSize StoreSize = DL.getTypeStoreSize(StoredValue->getType());
      if (StoreSize.isScalable()) {
        if (getUnderlyingObject(Store->getPointerOperand()) == Record.Alloca)
          Initialized.reset();
        continue;
      }

      bool WritesZero = false;
      if (auto *C = dyn_cast<Constant>(StoredValue))
        WritesZero = C->isNullValue();

      SmallVector<uint64_t, 4> StoredPointerOffsets;
      if (!WritesZero && !isa<UndefValue, PoisonValue>(StoredValue) &&
          containsPointer(StoredValue->getType())) {
        SmallVector<PointerFrameLeaf, 4> StoredLeaves;
        SmallVector<unsigned, 4> Path;
        if (Error Err = enumeratePointerFrameLeaves(StoredValue->getType(), DL,
                                                    Path, 0, StoredLeaves)) {
          consumeError(std::move(Err));
        } else {
          for (const PointerFrameLeaf &Leaf : StoredLeaves) {
            Value *LeafValue = FindInsertedValue(StoredValue, Leaf.Indices);
            // A non-constant aggregate SSA value is defined by the Go value
            // model even when FindInsertedValue cannot recover an individual
            // leaf. Explicit undef/poison construction remains uninitialized.
            if (!LeafValue || !isa<UndefValue, PoisonValue>(LeafValue))
              StoredPointerOffsets.push_back(Leaf.Offset);
          }
        }
      }
      updateInitializedPointerSlots(Initialized, Store->getPointerOperand(),
                                    StoreSize.getFixedValue(), WritesZero,
                                    StoredPointerOffsets, Record, DL);
      continue;
    }

    if (auto *Set = dyn_cast<MemSetInst>(I)) {
      auto *ByteValue = dyn_cast<ConstantInt>(Set->getValue());
      auto *Length = dyn_cast<ConstantInt>(Set->getLength());
      if (!Length || Length->getValue().getActiveBits() > 64) {
        if (getUnderlyingObject(Set->getDest()) == Record.Alloca)
          Initialized.reset();
        continue;
      }
      updateInitializedPointerSlots(
          Initialized, Set->getDest(), Length->getZExtValue(),
          ByteValue && ByteValue->isZero(), {}, Record, DL);
      continue;
    }
    if (auto *Transfer = dyn_cast<MemTransferInst>(I)) {
      auto *Length = dyn_cast<ConstantInt>(Transfer->getLength());
      if (!Length || Length->getValue().getActiveBits() > 64) {
        if (getUnderlyingObject(Transfer->getDest()) == Record.Alloca)
          Initialized.reset();
        continue;
      }
      updateInitializedPointerSlots(Initialized, Transfer->getDest(),
                                    Length->getZExtValue(), false, {}, Record,
                                    DL);
      continue;
    }
    if (auto *Call = dyn_cast<CallBase>(I);
        Call && !isTerminalTailTransfer(*Call) && !isLeafCall(*Call)) {
      if (auto *OrdinaryCall = dyn_cast<CallInst>(Call);
          OrdinaryCall && llvm::is_contained(Record.ActiveCalls, OrdinaryCall))
        return Initialized.count() == Record.Layout.Leaves.size();
      // On a normal return, a typed goret call has initialized the complete
      // logical result object. The caller stack map at that call describes
      // the old contents, so apply this definition only for later calls.
      if (auto *OrdinaryCall = dyn_cast<CallInst>(Call);
          OrdinaryCall && llvm::is_contained(Record.GoRetDefs, OrdinaryCall))
        Initialized.set();
    }
  }
  return Initialized.count() == Record.Layout.Leaves.size();
}

void preparePointerAllocaStorageForGC(
    MutableArrayRef<PointerAllocaRecord> PointerAllocas) {
  if (PointerAllocas.empty())
    return;
  const DataLayout &DL =
      PointerAllocas.front().Alloca->getFunction()->getDataLayout();
  for (PointerAllocaRecord &Record : PointerAllocas) {
    if (Record.WholeLifetime) {
      for (IntrinsicInst *Marker : Record.LifetimeMarkers)
        Marker->eraseFromParent();

      // Content activity above used the original VarDef intervals. Only now
      // widen and initialize the physical storage so every earlier bitmap that
      // can name it remains safe to scan.
      IRBuilder<> Builder(Record.Alloca->getNextNode());
      Builder.SetCurrentDebugLocation(Record.Alloca->getDebugLoc());
      Builder.CreateLifetimeStart(Record.Alloca);
      Builder.CreateMemSetInline(Record.Alloca, Record.Alloca->getAlign(),
                                 Builder.getInt8(0),
                                 Builder.getInt64(Record.Layout.ByteSize));
      Record.Alloca->setMetadata(
          StackColoringNoMergeMD,
          MDNode::get(Record.Alloca->getContext(), ArrayRef<Metadata *>()));
      continue;
    }
    if (Record.DeferResult || Record.OpenDeferSlot)
      continue;

    SmallVector<IntrinsicInst *, 4> LifetimeStarts;
    for (IntrinsicInst *Marker : Record.LifetimeMarkers)
      if (Marker->getIntrinsicID() == Intrinsic::lifetime_start)
        LifetimeStarts.push_back(Marker);

    auto InitializeAt = [&](Instruction *InsertBefore) {
      IRBuilder<> Builder(InsertBefore);
      Builder.SetCurrentDebugLocation(InsertBefore->getDebugLoc());
      Builder.CreateMemSetInline(Record.Alloca, Record.Alloca->getAlign(),
                                 Builder.getInt8(0),
                                 Builder.getInt64(Record.Layout.ByteSize));
    };
    if (LifetimeStarts.empty()) {
      Instruction *Begin = Record.Alloca->getNextNode();
      if (!hasInitializedPointerSlotsBeforeSafepoint(Begin, Record, DL))
        InitializeAt(Begin);
      continue;
    }
    for (IntrinsicInst *Start : LifetimeStarts)
      if (Instruction *Begin = Start->getNextNode();
          !hasInitializedPointerSlotsBeforeSafepoint(Begin, Record, DL))
        InitializeAt(Begin);
  }
}

Error collectPointerAllocas(
    Function &F, const std::optional<OpenDeferInfo> &OpenDefer,
    SmallVectorImpl<PointerAllocaRecord> &PointerAllocas) {
  bool HasSafepoint = llvm::any_of(instructions(F), [](Instruction &I) {
    auto *Call = dyn_cast<CallBase>(&I);
    return Call && !isTerminalTailTransfer(*Call) &&
           !isa<GCStatepointInst>(Call) && !isLeafCall(*Call);
  });
  if (!HasSafepoint)
    return Error::success();

  const DataLayout &DL = F.getDataLayout();
  for (Instruction &I : instructions(F)) {
    auto *Alloca = dyn_cast<AllocaInst>(&I);
    if (Alloca && isSingleByValCallCarrier(*Alloca, DL))
      continue;
    if (!Alloca || !containsPointer(Alloca->getAllocatedType()))
      continue;
    auto *ArraySize = dyn_cast<ConstantInt>(Alloca->getArraySize());
    if (!Alloca->isStaticAlloca() ||
        Alloca->getParent() != &F.getEntryBlock() || !ArraySize ||
        !ArraySize->isOne())
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints require a single fixed entry-block "
          "pointer-containing alloca");
    Expected<PointerFrameLayout> Layout =
        pointerFrameLayout(Alloca->getAllocatedType(), Alloca->getAlign(), DL,
                           PointerFrameKind::Alloca, DL.getStackAlignment());
    if (!Layout)
      return Layout.takeError();
    Expected<bool> DeferResult = deferResultAlloca(*Alloca);
    if (!DeferResult)
      return DeferResult.takeError();
    Alloca->setMetadata(GoDeferResultMD, nullptr);
    bool IsOpenDeferSlot = OpenDefer && OpenDefer->Slots == Alloca;
    // Do not override the ordinary structural StackObject classification for
    // open-defer state. Its explicit per-call contents-live bit makes GoObj
    // expand this layout into LocalsPointerMaps; an inactive callsite follows
    // the same StackObject rule as every other address-observable alloca.
    bool NeedsStackObject = addressNeedsStackObject(*Alloca);
    PointerAllocas.push_back({Alloca, NeedsStackObject, *DeferResult,
                              IsOpenDeferSlot, std::move(*Layout)});
  }

  if (Error Err = collectPointerAllocaLifetimeMarkers(F, PointerAllocas))
    return Err;

  return Error::success();
}

Error collectPointerFixedArgs(Function &F,
                              SmallVectorImpl<PointerFixedArgRecord> &Records) {
  if (!isGoCallingConv(F.getCallingConv()))
    return Error::success();

  const DataLayout &DL = F.getDataLayout();
  for (Argument &Arg : F.args()) {
    bool IsGoRet = Arg.hasGoRetAttr();
    if (!Arg.hasByValAttr() && !IsGoRet)
      continue;
    Type *StorageType =
        IsGoRet ? Arg.getParamGoRetType() : Arg.getParamByValType();
    if (!StorageType || !containsPointer(StorageType))
      continue;

    Align Alignment =
        Arg.getParamAlign().value_or(DL.getABITypeAlign(StorageType));
    Expected<PointerFrameLayout> Layout =
        pointerFrameLayout(StorageType, Alignment, DL,
                           PointerFrameKind::FixedArgument, std::nullopt);
    if (!Layout)
      return Layout.takeError();
    PointerFixedArgRecord Record;
    Record.Base = &Arg;
    Record.IsGoRet = IsGoRet;
    Record.NeedsStackObject = addressNeedsStackObject(Arg);
    Record.DeferResult = IsGoRet && Arg.hasAttribute(GoDeferResultMD);
    Record.Layout = std::move(*Layout);
    Records.push_back(std::move(Record));
  }
  return Error::success();
}

SmallBitVector &
fixedArgAccessMask(DenseMap<const Instruction *, SmallBitVector> &Masks,
                   Instruction &I, size_t BitCount) {
  SmallBitVector &Mask = Masks[&I];
  if (Mask.empty())
    Mask.resize(BitCount);
  return Mask;
}

void addAllFixedArgSlots(DenseMap<const Instruction *, SmallBitVector> &Masks,
                         Instruction &I, const PointerFixedArgRecord &Record) {
  fixedArgAccessMask(Masks, I, Record.Layout.Leaves.size()).set();
}

void addFixedArgMemorySlots(
    DenseMap<const Instruction *, SmallBitVector> &Masks, Instruction &I,
    Value *Address, uint64_t AccessSize, bool IsDefinition,
    PointerFixedArgRecord &Record) {
  const DataLayout &DL = I.getDataLayout();
  std::optional<int64_t> Offset =
      Address->getPointerOffsetFrom(Record.Base, DL);
  if (!Offset || *Offset < 0) {
    // Unknown reads may observe any pointer slot. Unknown writes cannot kill
    // any slot because no single slot is known to be overwritten. This is the
    // variable-level may-use/must-def conservative transfer.
    if (!IsDefinition)
      addAllFixedArgSlots(Masks, I, Record);
    return;
  }

  uint64_t AccessBegin = static_cast<uint64_t>(*Offset);
  std::optional<uint64_t> AccessEnd =
      checkedAddUnsigned(AccessBegin, AccessSize);
  if (!AccessEnd) {
    if (!IsDefinition)
      addAllFixedArgSlots(Masks, I, Record);
    return;
  }

  uint64_t PointerSize = DL.getPointerSize(0);
  SmallBitVector Mask(Record.Layout.Leaves.size());
  for (auto [Index, Leaf] : llvm::enumerate(Record.Layout.Leaves)) {
    uint64_t SlotBegin = Leaf.Offset;
    uint64_t SlotEnd = SlotBegin + PointerSize;
    if (*AccessEnd <= SlotBegin || AccessBegin >= SlotEnd)
      continue;
    if (IsDefinition && (AccessBegin > SlotBegin || *AccessEnd < SlotEnd)) {
      // A safepoint must not scan a partially overwritten pointer word.
      Record.ActivityUnclear = true;
      return;
    }
    Mask.set(Index);
  }
  if (Mask.any())
    fixedArgAccessMask(Masks, I, Record.Layout.Leaves.size()) |= Mask;
}

std::optional<uint64_t> fixedAccessSize(Type *Ty, const DataLayout &DL) {
  TypeSize Size = DL.getTypeStoreSize(Ty);
  if (Size.isScalable())
    return std::nullopt;
  return Size.getFixedValue();
}

void collectFixedArgContentAccesses(PointerFixedArgRecord &Record) {
  visitFixedFrameAddressUses(
      *Record.Base,
      [&](Value *Address, Use &U, Instruction *I, FrameAddressUseKind Kind) {
        if (!I) {
          Record.ActivityUnclear = true;
          return;
        }
        if (Kind == FrameAddressUseKind::Derivation) {
          Record.ActivityUnclear = true;
          return;
        }
        if (Kind == FrameAddressUseKind::LifetimeOrDebug)
          return;
        if (Kind == FrameAddressUseKind::FakeUse) {
          addAllFixedArgSlots(Record.ContentUses, *I, Record);
          return;
        }
        if (Kind == FrameAddressUseKind::FirstClass) {
          if (!isa<PHINode, SelectInst, FreezeInst, ICmpInst>(I))
            addAllFixedArgSlots(Record.ContentUses, *I, Record);
          return;
        }

        const DataLayout &DL = I->getDataLayout();
        if (auto *Load = dyn_cast<LoadInst>(I)) {
          std::optional<uint64_t> Size = fixedAccessSize(Load->getType(), DL);
          if (!Size)
            addAllFixedArgSlots(Record.ContentUses, *I, Record);
          else
            addFixedArgMemorySlots(Record.ContentUses, *I, Address, *Size,
                                   false, Record);
          return;
        }
        if (auto *Store = dyn_cast<StoreInst>(I)) {
          std::optional<uint64_t> Size =
              fixedAccessSize(Store->getValueOperand()->getType(), DL);
          if (Size)
            addFixedArgMemorySlots(Record.ContentDefs, *I, Address, *Size, true,
                                   Record);
          return;
        }

        Type *ReadWriteType = nullptr;
        if (auto *RMW = dyn_cast<AtomicRMWInst>(I))
          ReadWriteType = RMW->getValOperand()->getType();
        else if (auto *CmpXchg = dyn_cast<AtomicCmpXchgInst>(I))
          ReadWriteType = CmpXchg->getCompareOperand()->getType();
        if (ReadWriteType) {
          std::optional<uint64_t> Size = fixedAccessSize(ReadWriteType, DL);
          if (!Size) {
            addAllFixedArgSlots(Record.ContentUses, *I, Record);
          } else {
            addFixedArgMemorySlots(Record.ContentDefs, *I, Address, *Size, true,
                                   Record);
            addFixedArgMemorySlots(Record.ContentUses, *I, Address, *Size,
                                   false, Record);
          }
          return;
        }

        auto *Mem = dyn_cast<MemIntrinsic>(I);
        if (!Mem) {
          Record.ActivityUnclear = true;
          return;
        }
        bool IsDest = U.get() == Mem->getRawDest();
        auto *Transfer = dyn_cast<MemTransferInst>(Mem);
        bool IsSource = Transfer && U.get() == Transfer->getRawSource();
        if (!IsDest && !IsSource) {
          Record.ActivityUnclear = true;
          return;
        }
        auto *Length = dyn_cast<ConstantInt>(Mem->getLength());
        if (!Length || Length->getValue().getActiveBits() > 64) {
          if (IsSource)
            addAllFixedArgSlots(Record.ContentUses, *I, Record);
          return;
        }
        uint64_t Size = Length->getZExtValue();
        if (IsDest)
          addFixedArgMemorySlots(Record.ContentDefs, *I, Address, Size, true,
                                 Record);
        else
          addFixedArgMemorySlots(Record.ContentUses, *I, Address, Size, false,
                                 Record);
      });
}

void transferByValContentLiveness(const PointerFixedArgRecord &Record,
                                  const Instruction &I, SmallBitVector &Live) {
  assert(!Record.IsGoRet && "goret uses forward initialization activity");
  if (auto It = Record.ContentDefs.find(&I); It != Record.ContentDefs.end())
    Live.reset(It->second);
  if (auto It = Record.ContentUses.find(&I); It != Record.ContentUses.end())
    Live |= It->second;
}

SmallBitVector byValContentsLiveInBlock(const PointerFixedArgRecord &Record,
                                        const BasicBlock &BB,
                                        SmallBitVector Live) {
  for (const Instruction &I : llvm::reverse(BB)) {
    transferByValContentLiveness(Record, I, Live);
  }
  return Live;
}

void transferGoRetInitialization(const PointerFixedArgRecord &Record,
                                 const Instruction &I,
                                 SmallBitVector &Initialized) {
  assert(Record.IsGoRet && "byval uses backward content liveness");
  if (auto It = Record.ContentDefs.find(&I); It != Record.ContentDefs.end())
    Initialized |= It->second;
}

void computeByValContentActivity(
    Function &F, PointerFixedArgRecord &Record,
    const SmallPtrSetImpl<const CallInst *> &SafepointCalls,
    const DominatorTree &DT) {
  DenseMap<const BasicBlock *, SmallBitVector> LiveIn;
  bool Changed;
  do {
    Changed = false;
    for (BasicBlock &BB : llvm::reverse(F)) {
      SmallBitVector LiveOut(Record.Layout.Leaves.size());
      for (BasicBlock *Succ : successors(&BB))
        if (auto It = LiveIn.find(Succ); It != LiveIn.end())
          LiveOut |= It->second;
      SmallBitVector NewLiveIn =
          byValContentsLiveInBlock(Record, BB, std::move(LiveOut));
      auto It = LiveIn.find(&BB);
      if (It == LiveIn.end() || NewLiveIn != It->second) {
        LiveIn[&BB] = NewLiveIn;
        Changed = true;
      }
    }
  } while (Changed);

  for (BasicBlock &BB : F) {
    SmallBitVector Live(Record.Layout.Leaves.size());
    for (BasicBlock *Succ : successors(&BB))
      if (auto It = LiveIn.find(Succ); It != LiveIn.end())
        Live |= It->second;
    for (Instruction &I : llvm::reverse(BB)) {
      auto *Call = dyn_cast<CallInst>(&I);
      if (Call && SafepointCalls.contains(Call) &&
          (Live.any() || !DT.isReachableFromEntry(Call->getParent())))
        Record.ActiveCalls.push_back(Call);
      transferByValContentLiveness(Record, I, Live);
    }
  }
}

void computeGoRetContentActivity(
    Function &F, PointerFixedArgRecord &Record,
    const SmallPtrSetImpl<const CallInst *> &SafepointCalls,
    const DominatorTree &DT) {
  const size_t SlotCount = Record.Layout.Leaves.size();
  DenseMap<const BasicBlock *, SmallBitVector> InitializedIn;
  DenseMap<const BasicBlock *, SmallBitVector> InitializedOut;

  // Definite initialization is a forward must analysis. Start reachable
  // non-entry blocks at top and intersect predecessor states; the entry starts
  // with no initialized result contents.
  for (BasicBlock &BB : F) {
    bool IsEntry = &BB == &F.getEntryBlock();
    bool Reachable = DT.isReachableFromEntry(&BB);
    InitializedIn[&BB] = SmallBitVector(SlotCount, Reachable && !IsEntry);
    InitializedOut[&BB] = InitializedIn[&BB];
    for (Instruction &I : BB)
      transferGoRetInitialization(Record, I, InitializedOut[&BB]);
  }

  bool Changed;
  do {
    Changed = false;
    for (BasicBlock &BB : F) {
      SmallBitVector NewIn(SlotCount);
      if (&BB != &F.getEntryBlock() && DT.isReachableFromEntry(&BB)) {
        bool FirstPredecessor = true;
        for (BasicBlock *Pred : predecessors(&BB)) {
          if (!DT.isReachableFromEntry(Pred))
            continue;
          if (FirstPredecessor) {
            NewIn = InitializedOut.lookup(Pred);
            FirstPredecessor = false;
          } else {
            NewIn &= InitializedOut.lookup(Pred);
          }
        }
      }

      SmallBitVector NewOut = NewIn;
      for (Instruction &I : BB)
        transferGoRetInitialization(Record, I, NewOut);
      if (NewIn != InitializedIn.lookup(&BB) ||
          NewOut != InitializedOut.lookup(&BB)) {
        InitializedIn[&BB] = std::move(NewIn);
        InitializedOut[&BB] = std::move(NewOut);
        Changed = true;
      }
    }
  } while (Changed);

  for (BasicBlock &BB : F) {
    SmallBitVector Initialized = InitializedIn.lookup(&BB);
    for (Instruction &I : BB) {
      auto *Call = dyn_cast<CallInst>(&I);
      bool FullyInitialized = Initialized.count() == SlotCount;
      if (Call && SafepointCalls.contains(Call) &&
          (Record.DeferResult || FullyInitialized ||
           !DT.isReachableFromEntry(Call->getParent())))
        Record.ActiveCalls.push_back(Call);
      transferGoRetInitialization(Record, I, Initialized);
    }
  }
}

Error computePointerFixedArgActivity(
    Function &F, MutableArrayRef<PointerFixedArgRecord> Records,
    const SmallPtrSetImpl<const CallInst *> &SafepointCalls,
    const DominatorTree &DT) {
  for (PointerFixedArgRecord &Record : Records) {
    collectFixedArgContentAccesses(Record);
    if (Record.ActivityUnclear)
      return createStringError(
          std::errc::not_supported,
          "GoALLC cannot determine fixed argument content activity");

    if (Record.IsGoRet)
      computeGoRetContentActivity(F, Record, SafepointCalls, DT);
    else
      computeByValContentActivity(F, Record, SafepointCalls, DT);
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
  if (Call.getNumOperandBundles() != 0 &&
      (Call.getNumOperandBundles() != 1 ||
       !Call.getOperandBundle(LLVMContext::OB_deopt)))
    return createStringError(
        std::errc::not_supported,
        "GoALLC statepoints only support a single deopt call operand bundle");
  for (unsigned I = 0; I != Call.arg_size(); ++I) {
    if (Call.paramHasAttr(I, Attribute::ByVal) &&
        (!isGoCallingConv(Call.getCallingConv()) ||
         !Call.getArgOperand(I)->getType()->isPointerTy() ||
         !Call.getParamByValType(I) || !Call.getParamAlign(I)))
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints require typed, aligned byval only on Go calls");
    if (Call.paramHasAttr(I, Attribute::GoRet) &&
        (!isGoCallingConv(Call.getCallingConv()) ||
         !Call.getArgOperand(I)->getType()->isPointerTy() ||
         !Call.getParamGoRetType(I) ||
         !Call.getParamAttr(I, "goretindex").isValid() ||
         !Call.getParamAlign(I)))
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints require indexed, typed, aligned goret only on "
          "Go calls");
    for (Attribute Attr : Call.getAttributes().getParamAttrs(I)) {
      // These non-ABI attributes remain valid after LLVM's generic
      // RewriteStatepointsForGC pass and are natively accepted by both the
      // statepoint verifier and SelectionDAG call lowering. O2 commonly
      // infers them on otherwise ordinary runtime calls. Keep ABI-affecting
      // attributes fail closed except for nest and the typed Go ABI memory
      // carriers, whose lowering is covered separately.
      if (!Attr.hasAttribute(Attribute::Nest) &&
          !Attr.hasAttribute(Attribute::ByVal) &&
          !Attr.hasAttribute(Attribute::GoRet) &&
          !Attr.hasAttribute("goretindex") &&
          !Attr.hasAttribute(Attribute::Captures) &&
          !Attr.hasAttribute(Attribute::ReadNone) &&
          !Attr.hasAttribute(Attribute::ReadOnly) &&
          !Attr.hasAttribute(Attribute::NonNull) &&
          !Attr.hasAttribute(Attribute::NoUndef) &&
          !Attr.hasAttribute(Attribute::Alignment))
        return createStringError(
            std::errc::not_supported,
            "GoALLC statepoints do not support call parameter attribute '%s'",
            Attr.getAsString().c_str());
    }
  }
  for (Value *V : Record.Live) {
    if (!isRelocatablePointerType(V->getType()))
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints do not yet support live pointer aggregates");
  }
  return Error::success();
}

void appendAllocaPtrMapDeoptOperands(
    IRBuilder<> &Builder, ArrayRef<const PointerAllocaRecord *> Allocas,
    ArrayRef<const PointerFixedArgRecord *> FixedArgs,
    const SmallPtrSetImpl<Value *> &LiveContents,
    SmallVectorImpl<Value *> &Deopt) {
  if (Allocas.empty() && FixedArgs.empty())
    return;
  // ProtocolLength covers BEGIN through END, but not the trailing duplicate
  // length.  The envelope itself therefore contributes BEGIN, length,
  // record-count, and END.
  uint64_t ProtocolLength = 4;
  for (const PointerAllocaRecord *Alloca : Allocas)
    ProtocolLength += 11 + Alloca->Layout.BitmapWords.size();
  for (const PointerFixedArgRecord *FixedArg : FixedArgs)
    ProtocolLength += 11 + FixedArg->Layout.BitmapWords.size();

  auto AppendConstant = [&](uint64_t Value) {
    Deopt.push_back(ConstantInt::get(Builder.getInt64Ty(), Value));
  };
  AppendConstant(GoObj::AllocaPtrMapBeginMagic);
  AppendConstant(ProtocolLength);
  AppendConstant(Allocas.size() + FixedArgs.size());
  auto AppendRecord = [&](Value *Base, const PointerFrameLayout &Layout) {
    AppendConstant(GoObj::AllocaPtrMapRecordTag);
    AppendConstant(11 + Layout.BitmapWords.size());
    Deopt.push_back(Base);
    AppendConstant(0); // First contract version describes the whole object.
    AppendConstant(Layout.ByteSize);
    AppendConstant(Layout.Alignment);
    AppendConstant(
        Builder.GetInsertBlock()->getModule()->getDataLayout().getPointerSize(
            0));
    // gc-live also carries direct frame bases needed only to rematerialize an
    // address after stack growth. Keep object-content liveness independent so
    // GoObj never mistakes that address-only operand for a LocalsPointerMaps
    // root.
    AppendConstant(LiveContents.contains(Base));
    AppendConstant(Layout.BitCount);
    AppendConstant(GoObj::AllocaPtrMapBitmapWordBits);
    AppendConstant(Layout.BitmapWords.size());
    for (uint64_t Word : Layout.BitmapWords)
      AppendConstant(Word);
  };
  for (const PointerAllocaRecord *Alloca : Allocas)
    AppendRecord(Alloca->Alloca, Alloca->Layout);
  for (const PointerFixedArgRecord *FixedArg : FixedArgs)
    AppendRecord(FixedArg->Base, FixedArg->Layout);
  AppendConstant(GoObj::AllocaPtrMapEndMagic);
  AppendConstant(ProtocolLength);
}

void appendOpenDeferDeoptOperands(IRBuilder<> &Builder,
                                  const std::optional<OpenDeferInfo> &OpenDefer,
                                  SmallVectorImpl<Value *> &Deopt) {
  if (!OpenDefer)
    return;
  constexpr uint64_t ProtocolLength = 6;
  auto AppendConstant = [&](uint64_t Value) {
    Deopt.push_back(ConstantInt::get(Builder.getInt64Ty(), Value));
  };
  AppendConstant(GoObj::OpenDeferBeginMagic);
  AppendConstant(ProtocolLength);
  AppendConstant(OpenDefer->SlotCount);
  Deopt.push_back(OpenDefer->Bits);
  Deopt.push_back(OpenDefer->Slots);
  AppendConstant(GoObj::OpenDeferEndMagic);
  AppendConstant(ProtocolLength);
}

Error rewriteCall(SafepointRecord &Record,
                  ArrayRef<const PointerAllocaRecord *> PointerAllocas,
                  ArrayRef<const PointerFixedArgRecord *> PointerFixedArgs,
                  const SmallPtrSetImpl<Value *> &LiveContents,
                  const std::optional<OpenDeferInfo> &OpenDefer) {
  CallInst *Call = Record.Call;

  SmallVector<Value *, 8> CallArgs(Call->args());
  SmallVector<Value *, 8> GCLive(Record.Live.begin(), Record.Live.end());
  SmallVector<Value *, 32> Deopt;
  if (auto Bundle = Call->getOperandBundle(LLVMContext::OB_deopt))
    for (const Use &Input : Bundle->Inputs)
      Deopt.push_back(Input.get());
  FunctionCallee Callee(Call->getFunctionType(), Call->getCalledOperand());

  IRBuilder<> Builder(Call);
  Builder.SetCurrentDebugLocation(Call->getDebugLoc());
  // Keep the open-defer envelope before the alloca ptrmap envelope. The latter
  // deliberately remains the final self-describing suffix for compatibility.
  appendOpenDeferDeoptOperands(Builder, OpenDefer, Deopt);
  appendAllocaPtrMapDeoptOperands(Builder, PointerAllocas, PointerFixedArgs,
                                  LiveContents, Deopt);
  Record.Statepoint = Builder.CreateGCStatepointCall(
      Record.ID, 0, Callee, CallArgs,
      Deopt.empty() ? std::nullopt
                    : std::optional<ArrayRef<Value *>>(ArrayRef(Deopt)),
      GCLive, "statepoint_token");
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
    // A static alloca or typed byval/goret argument in gc-live describes an
    // original frame object to the statepoint stack map. Its address is not a
    // movable heap pointer and must not acquire a gc.relocate SSA identity.
    // SelectionDAG records the FrameIndex directly; GoObj combines that
    // location with any independent pointer-map record for live contents.
    if (isFixedFrameBase(V))
      continue;
    CallInst *Relocate = Builder.CreateGCRelocate(
        Record.Statepoint, Index, Index, V->getType(),
        V->hasName() ? V->getName() + ".relocated" : "");
    Relocate->setCallingConv(CallingConv::Cold);
    Record.Relocates.push_back(Relocate);
  }
  for (const auto &Coalesced : Record.CoalescedLiveRoots) {
    Value *Alias = Coalesced.first;
    Value *Canonical = Coalesced.second;
    auto Relocate = llvm::find_if(Record.Relocates, [&](CallInst *Call) {
      return cast<GCRelocateInst>(Call)->getDerivedPtr() == Canonical;
    });
    assert(Relocate != Record.Relocates.end() &&
           "coalesced aggregate alias root is missing its relocate");
    Record.CoalescedRelocates.push_back({Alias, *Relocate});
  }

  Record.Result = Result;
  return Error::success();
}

void eraseOriginalCalls(ArrayRef<SafepointRecord> Records) {
  // Keep every original call and its result alive until all precomputed
  // liveness sets have been consumed. LLVM block layout need not follow CFG
  // dominance, so a record encountered earlier can legitimately contain the
  // result of a call encountered later. Replacing and erasing each call inside
  // rewriteCall would leave that record with a dangling Value pointer.
  for (const SafepointRecord &Record : Records) {
    if (Record.Result)
      Record.Call->replaceAllUsesWith(Record.Result);
    Record.Call->eraseFromParent();
  }
}

void splitStatepointContinuations(ArrayRef<SafepointRecord> Records) {
  // Do this only after every call has been rewritten and erased, so CFG
  // mutation cannot affect the liveness sets consumed by rewriteCall. Keep
  // each statepoint and all values derived from its token in distinct basic
  // blocks: SelectionDAG performs local CSE while lowering one block, and the
  // explicit edge puts gc.result and gc.relocate in a fresh local CSE scope.
  for (const SafepointRecord &Record : Records) {
    Instruction *Continuation = Record.Statepoint->getNextNode();
    assert(Continuation && "statepoint must have a continuation instruction");
    BasicBlock *StatepointBlock = Record.Statepoint->getParent();
    StatepointBlock->splitBasicBlock(Continuation->getIterator(),
                                     StatepointBlock->getName() +
                                         ".statepoint.cont");
  }
}

Value *rematerializeAddress(Value *Address, Value *Base, Value *RelocatedBase,
                            Instruction *InsertBefore) {
  SmallVector<Instruction *, 4> Chain;
  Value *Current = Address;
  while (Current != Base) {
    auto *I = cast<Instruction>(Current);
    assert((isa<GetElementPtrInst>(I) || isa<CastInst>(I)) &&
           "unexpected rematerializable address");
    Chain.push_back(I);
    Current = isa<GetElementPtrInst>(I)
                  ? cast<GetElementPtrInst>(I)->getPointerOperand()
                  : cast<CastInst>(I)->getOperand(0);
  }

  Value *OldOperand = Current;
  Value *NewOperand = RelocatedBase;
  for (Instruction *I : llvm::reverse(Chain)) {
    auto *Clone = I->clone();
    Clone->replaceUsesOfWith(OldOperand, NewOperand);
    Clone->setName(I->hasName() ? I->getName() + ".remat" : "address.remat");
    Clone->insertBefore(InsertBefore->getIterator());
    OldOperand = I;
    NewOperand = Clone;
  }
  return NewOperand;
}

void repairRelocationSSA(Function &F, DominatorTree &DT,
                         ArrayRef<SafepointRecord> Records) {
  // Each ordinary relocated pointer and each rematerialized derived address is
  // a new reaching definition of its original SSA value. Fixed frame
  // identities never enter this map: their uses are rebuilt directly from the
  // original FrameIndex.
  MapVector<Value *, SmallVector<Value *, 4>> Definitions;
  for (const SafepointRecord &Record : Records) {
    for (CallInst *RelocateCall : Record.Relocates) {
      auto *Relocate = cast<GCRelocateInst>(RelocateCall);
      Definitions[Relocate->getDerivedPtr()].push_back(RelocateCall);
    }
    for (const auto &Coalesced : Record.CoalescedRelocates) {
      Value *Alias = Coalesced.first;
      // An original call can become unused after all aggregate uses have been
      // rebuilt. eraseOriginalCalls then deletes it without a replacement,
      // and there is no remaining alias SSA use to repair.
      if (!Alias)
        continue;
      Definitions[Alias].push_back(Coalesced.second);
    }

    Instruction *InsertBefore = Record.Relocates.empty()
                                    ? Record.Statepoint->getNextNode()
                                    : Record.Relocates.back()->getNextNode();
    for (Value *Address : Record.DerivedPointers) {
      Value *Base = rematerializableDerivedBase(Address);
      auto Relocate = llvm::find_if(Record.Relocates, [&](CallInst *Call) {
        return cast<GCRelocateInst>(Call)->getDerivedPtr() == Base;
      });
      assert(Relocate != Record.Relocates.end() &&
             "derived pointer is missing its base relocate");
      Value *Rematerialized =
          rematerializeAddress(Address, Base, *Relocate, InsertBefore);
      Definitions[Address].push_back(Rematerialized);
    }
  }
  if (Definitions.empty())
    return;

  const DataLayout &DL = F.getDataLayout();
  MapVector<Value *, AllocaInst *> Slots;
  SmallVector<AllocaInst *, 16> PromotableAllocas;
  PromotableAllocas.reserve(Definitions.size());
  for (auto &[V, NewDefinitions] : Definitions) {
    (void)NewDefinitions;
    StringRef Name = V->hasName() ? V->getName() : "pointer";
    auto *Slot = new AllocaInst(V->getType(), DL.getAllocaAddrSpace(),
                                (Name + ".relocated.merge").str(),
                                F.getEntryBlock().getFirstNonPHIIt());
    Slots[V] = Slot;
    PromotableAllocas.push_back(Slot);
  }

  for (auto &[Original, NewDefinitions] : Definitions) {
    AllocaInst *Slot = Slots.lookup(Original);
    for (Value *Definition : NewDefinitions) {
      auto *I = cast<Instruction>(Definition);
      new StoreInst(Definition, Slot, std::next(I->getIterator()));
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
      if (isa<GCStatepointInst>(Call) ||
          (!isTerminalTailTransfer(*Call) && !isLeafCall(*Call)))
        return createStringError(
            std::errc::invalid_argument,
            "GoALLC gc-leaf-function contains a non-leaf call");
    }
    return Error::success();
  }

  DominatorTree DT(F);
  LoopInfo LI(DT);
  Expected<std::optional<OpenDeferInfo>> OpenDeferOrErr =
      collectOpenDeferInfo(F);
  if (!OpenDeferOrErr)
    return OpenDeferOrErr.takeError();
  std::optional<OpenDeferInfo> OpenDefer = std::move(*OpenDeferOrErr);
  if (OpenDefer) {
    OpenDefer->Bits->setMetadata(
        StackColoringNoMergeMD,
        MDNode::get(F.getContext(), ArrayRef<Metadata *>()));
  }
  MapVector<Value *, StatepointPreservationPlan> PreservationPlans =
      buildStatepointPreservationPlans(F, LI, DT);
  SmallVector<WeakTrackingVH, 16> AggregateScalarizationCandidates =
      collectRelocatablePointerAggregates(PreservationPlans);
  // Defer scalar pointer homes until aggregate scalarization has exposed
  // interface/slice/string pointer leaves, so every pointer uses the same
  // preservation policy.
  for (auto &[V, Plan] : PreservationPlans)
    if (V->getType()->isPointerTy())
      Plan.Strategy = StatepointPreservationStrategy::RelocateSSA;
  if (Error Err =
          applyStatepointPreservationPlans(PreservationPlans, F, DT, LI))
    return Err;
  // Selected homes can delete projection instructions which also appeared in
  // the original plan map. Candidate handles above track the aggregate values
  // still requiring scalarization; discard raw plan keys before continuing.
  PreservationPlans.clear();

  // Preserve the established frame/root analysis order. Aggregate
  // scalarization changes SSA projections and must not retroactively alter how
  // pre-existing allocas and fixed arguments are classified. New scalar homes
  // are collected separately and appended below.
  SmallVector<PointerAllocaRecord, 8> PointerAllocas;
  if (Error Err = collectPointerAllocas(F, OpenDefer, PointerAllocas))
    return Err;
  SmallVector<PointerFixedArgRecord, 4> PointerFixedArgs;
  if (Error Err = collectPointerFixedArgs(F, PointerFixedArgs))
    return Err;
  SmallPtrSet<AllocaInst *, 16> ExistingPointerAllocas;
  for (const PointerAllocaRecord &Record : PointerAllocas)
    ExistingPointerAllocas.insert(Record.Alloca);

  MapVector<Value *, Value *> DirectPointerLeafSources;
  if (Error Err = scalarizeLivePointerAggregates(
          F, AggregateScalarizationCandidates, DirectPointerLeafSources))
    return Err;

  // The first planning pass sees an interface, slice, or string as one SSA
  // aggregate. Scalarization exposes its pointer leaves, so run the same
  // pointer policy once more instead of maintaining a separate aggregate
  // liveness heuristic. Whole aggregates remain on their already-selected
  // path; only newly visible scalar pointers may acquire a loop home here.
  MapVector<Value *, StatepointPreservationPlan> ScalarizedPlans =
      buildStatepointPreservationPlans(F, LI, DT);
  for (auto &[V, Plan] : ScalarizedPlans)
    if (!V->getType()->isPointerTy())
      Plan.Strategy = StatepointPreservationStrategy::RelocateSSA;
  retainProfitablePointerHomes(ScalarizedPlans, DT);
  if (Error Err = applyStatepointPreservationPlans(ScalarizedPlans, F, DT, LI))
    return Err;
  ScalarizedPlans.clear();

  SmallVector<PointerAllocaRecord, 8> PointerAllocasAfterHomes;
  if (Error Err = collectPointerAllocas(F, OpenDefer, PointerAllocasAfterHomes))
    return Err;
  for (PointerAllocaRecord &Record : PointerAllocasAfterHomes)
    if (!ExistingPointerAllocas.contains(Record.Alloca))
      PointerAllocas.push_back(std::move(Record));
  if (Error Err = normalizeMergedDerivedPointers(F))
    return Err;
  // Scalarization can expose fixed-frame pointer PHIs/selects. Preserve only
  // their integer object offsets across statepoints; concrete addresses are
  // rebuilt from the canonical alloca/byval/goret base after continuations are
  // split into independent SelectionDAG blocks.
  Expected<SmallVector<FixedFrameAddressRecord, 32>> FixedFrameAddressesOrErr =
      prepareFixedFrameAddresses(F);
  if (!FixedFrameAddressesOrErr)
    return FixedFrameAddressesOrErr.takeError();
  SmallVector<FixedFrameAddressRecord, 32> FixedFrameAddresses =
      std::move(*FixedFrameAddressesOrErr);

  // Aggregate preservation has now been decided and materialized. Compute one
  // final SSA live set, then choose the representation-specific repair for
  // each value at the callsite. Pointer-containing memory has independent
  // content liveness and is added below by the alloca/fixed-argument analyses.
  LivenessData FinalLiveness = computeStatepointLiveness(F);
  SmallVector<SafepointRecord, 8> Records;
  uint64_t CallOrdinal = 0;

  for (Instruction &I : instructions(F)) {
    auto *Call = dyn_cast<CallBase>(&I);
    if (!Call || isa<GCStatepointInst>(Call) || isLeafCall(*Call) ||
        isTerminalTailTransfer(*Call))
      continue;
    auto *OrdinaryCall = dyn_cast<CallInst>(Call);
    if (!OrdinaryCall)
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints do not yet support invoke or callbr");
    SafepointRecord Record{OrdinaryCall,
                           stableStatepointID(F.getName(), CallOrdinal++)};
    for (Value *Live : liveAtCall(*OrdinaryCall, FinalLiveness)) {
      if (isFixedFrameAddress(Live)) {
        // Address liveness is independent from object-content liveness. Map
        // every same-object alloca/byval/goret recipe to its canonical frame
        // base below; fixed addresses never get gc.relocate.
        Record.FixedFrameAddresses.insert(Live);
      } else if (rematerializableDerivedBase(Live)) {
        Record.DerivedPointers.insert(Live);
      } else if (isOrdinaryRelocatablePointer(Live)) {
        Record.Live.insert(Live);
      } else if (isPointerAggregateValue(Live)) {
        return createStringError(
            std::errc::invalid_argument,
            "GoALLC statepoint preservation left a pointer-bearing "
            "aggregate live after scalarization");
      } else {
        return createStringError(
            std::errc::invalid_argument,
            "GoALLC statepoint liveness found an unsupported pointer value");
      }
    }
    Records.push_back(std::move(Record));
  }
  SmallPtrSet<const CallInst *, 16> SafepointCalls;
  for (const SafepointRecord &Record : Records)
    SafepointCalls.insert(Record.Call);
  if (Error Err = computePointerFixedArgActivity(F, PointerFixedArgs,
                                                 SafepointCalls, DT))
    return Err;
  if (Error Err =
          computePointerAllocaActivity(F, PointerAllocas, SafepointCalls, DT))
    return Err;
  DirectPointerLeafAliasGroups DirectPointerLeafAliases =
      buildDirectPointerLeafAliasGroups(DirectPointerLeafSources);
  for (SafepointRecord &Record : Records) {
    coalesceDirectAggregateLeafRoots(Record, DirectPointerLeafAliases);
    for (Value *Address : Record.FixedFrameAddresses) {
      Value *Base = fixedFrameProvenanceBase(Address);
      assert(Base && "live fixed frame address has no fixed base");
      auto AllocaIt =
          llvm::find_if(PointerAllocas, [&](const PointerAllocaRecord &Alloca) {
            return Alloca.Alloca == Base;
          });
      // An unobservable caller-side goret carrier is defined by this call.
      // Its IR pointer is neither a pre-call root nor an escaped address; the
      // statepoint's typed goret operand is sufficient for SelectionDAG to
      // materialize and then consume the ABI result area directly.
      if (AllocaIt != PointerAllocas.end() && !AllocaIt->NeedsStackObject &&
          !AllocaIt->DeferResult && !AllocaIt->OpenDeferSlot &&
          llvm::is_contained(AllocaIt->GoRetDefs, Record.Call))
        continue;
      Record.Live.insert(Base);
    }
    for (Value *Address : Record.DerivedPointers) {
      // A hoisted GEP from null can be a small non-Go pointer. Keep only its
      // relocatable base in Go's stack map; repairRelocationSSA reconstructs
      // the exact address expression after the statepoint.
      Record.Live.insert(rematerializableDerivedBase(Address));
    }
  }
  for (const SafepointRecord &Record : Records)
    if (Error Err = validateSafepoint(Record))
      return Err;
  protectStackObjectsFromColoring(PointerAllocas);
  preparePointerAllocaStorageForGC(PointerAllocas);
  for (SafepointRecord &Record : llvm::reverse(Records)) {
    SmallVector<const PointerAllocaRecord *, 8> AllocaRecords;
    SmallPtrSet<Value *, 8> LiveContents;
    for (const PointerAllocaRecord &Alloca : PointerAllocas) {
      // A recovered panic resumes outside LLVM's explicit CFG. The frontend
      // marks named result homes whose contents must therefore remain visible
      // to Go's stack scanner at every possible suspension call.
      bool ContentsLive = Alloca.DeferResult || Alloca.OpenDeferSlot ||
                          llvm::is_contained(Alloca.ActiveCalls, Record.Call);
      if (ContentsLive) {
        Record.Live.insert(Alloca.Alloca);
        LiveContents.insert(Alloca.Alloca);
      }
      // Address-observable layouts are function-wide metadata, so carry them
      // at every ordinary statepoint. The independent contents-live bit says
      // whether the object contributes roots at this call; an inactive record
      // lets GoObj infer the function-level StackObject set.
      if (ContentsLive || Alloca.NeedsStackObject)
        AllocaRecords.push_back(&Alloca);
    }
    SmallVector<const PointerFixedArgRecord *, 4> FixedArgRecords;
    for (const PointerFixedArgRecord &FixedArg : PointerFixedArgs) {
      // Byval inputs use ordinary backward content liveness: later reads
      // generate activity and definite overwrites kill the previous value.
      // Goret output homes instead become active after definite whole-variable
      // initialization and stay active through return; later stores update the
      // active contents rather than ending the interval. Keep both independent
      // from address liveness. At the requested variable granularity, an active
      // object contributes its complete typed bitmap to ArgsPointerMaps.
      // Address-observable layouts still describe function-level StackObjects
      // at every call.
      bool IsActive = llvm::is_contained(FixedArg.ActiveCalls, Record.Call);
      if (IsActive) {
        Record.Live.insert(FixedArg.Base);
        LiveContents.insert(FixedArg.Base);
      }
      if (IsActive || FixedArg.NeedsStackObject)
        FixedArgRecords.push_back(&FixedArg);
    }
    if (Error Err = rewriteCall(Record, AllocaRecords, FixedArgRecords,
                                LiveContents, OpenDefer))
      return Err;
  }
  eraseOriginalCalls(Records);
  splitStatepointContinuations(Records);
  if (Error Err = localizeFixedFrameAddresses(FixedFrameAddresses))
    return Err;
  // splitStatepointContinuations changes the CFG after liveness and
  // object-activity analysis.
  // General relocation repair promotes temporary merge slots, so rebuild the
  // tree for the new continuation blocks and localized fixed-frame uses.
  DT.recalculate(F);
  repairRelocationSSA(F, DT, Records);
  return Error::success();
}

Error materializeFunctionMarkerRelocs(Module &M) {
  SmallVector<Instruction *, 16> Markers;
  NamedMDNode *Relocs = nullptr;
  for (Function &F : M) {
    for (Instruction &I : instructions(F)) {
      auto *II = dyn_cast<IntrinsicInst>(&I);
      if (!II || II->getIntrinsicID() != Intrinsic::sideeffect)
        continue;
      MDNode *MD = I.getMetadata(GoObjMarkerRelocMD);
      if (!MD)
        continue;
      if (MD->getNumOperands() != 3)
        return createStringError(
            std::errc::invalid_argument,
            "GoALLC function marker relocation metadata must have three "
            "operands");
      auto *TargetMD = dyn_cast<ValueAsMetadata>(MD->getOperand(0));
      auto *Target =
          TargetMD ? dyn_cast<GlobalValue>(TargetMD->getValue()) : nullptr;
      auto *Type = mdconst::dyn_extract<ConstantInt>(MD->getOperand(1));
      auto *Addend = mdconst::dyn_extract<ConstantInt>(MD->getOperand(2));
      if (!Target || !Type || Type->getValue().ugt(UINT16_MAX) || !Addend)
        return createStringError(
            std::errc::invalid_argument,
            "GoALLC function marker relocation metadata is invalid");
      switch (Type->getZExtValue()) {
      case GoObj::R_USEIFACE:
      case GoObj::R_USEIFACEMETHOD:
      case GoObj::R_USENAMEDMETHOD:
      case GoObj::R_INITORDER:
        break;
      default:
        return createStringError(
            std::errc::invalid_argument,
            "GoALLC function marker relocation metadata has an unsupported "
            "type");
      }
      if (!Relocs)
        Relocs = M.getOrInsertNamedMetadata("goobj.marker_relocs");
      Metadata *Operands[] = {ValueAsMetadata::get(&F), TargetMD,
                              MD->getOperand(1).get(), MD->getOperand(2).get()};
      Relocs->addOperand(MDNode::get(M.getContext(), Operands));
      Markers.push_back(&I);
    }
  }
  for (Instruction *Marker : Markers)
    Marker->eraseFromParent();
  return Error::success();
}

void lowerPointerAddressConversions(Function &F) {
  SmallVector<IntrinsicInst *, 8> Calls;
  for (Instruction &I : instructions(F)) {
    auto *Call = dyn_cast<IntrinsicInst>(&I);
    if (!Call)
      continue;
    switch (Call->getIntrinsicID()) {
    case Intrinsic::go_pointer_address:
    case Intrinsic::go_pointer_from_address:
      Calls.push_back(Call);
      break;
    default:
      break;
    }
  }

  for (IntrinsicInst *Call : Calls) {
    IRBuilder<> Builder(Call);
    Builder.SetCurrentDebugLocation(Call->getDebugLoc());
    Value *Lowered = nullptr;
    if (Call->getIntrinsicID() == Intrinsic::go_pointer_address)
      Lowered = Builder.CreatePtrToInt(Call->getArgOperand(0), Call->getType(),
                                       Call->getName() + ".lowered");
    else
      Lowered = Builder.CreateIntToPtr(Call->getArgOperand(0), Call->getType(),
                                       Call->getName() + ".lowered");
    Call->replaceAllUsesWith(Lowered);
    Call->eraseFromParent();
  }
}

} // namespace

Error goallc::prepareStatepointModule(Module &M) {
  if (Error Err = materializeFunctionMarkerRelocs(M))
    return Err;
  return Error::success();
}

Error goallc::rewriteStatepoints(Function &F, TargetMachine &) {
  if (F.isDeclaration())
    return Error::success();
  lowerPointerAddressConversions(F);
  if (!isGoCallingConv(F.getCallingConv()) || !F.hasGC() ||
      F.getGC() != GoALLCGCName)
    return Error::success();
  return rewriteFunction(F);
}

Error goallc::rewriteStatepoints(Module &M, TargetMachine &TM) {
  if (Error Err = prepareStatepointModule(M))
    return Err;
  for (Function &F : M) {
    if (Error Err = rewriteStatepoints(F, TM))
      return Err;
  }
  if (verifyModule(M, &errs()))
    return createStringError(std::errc::invalid_argument,
                             "GoALLC statepoint rewrite produced invalid IR");
  return Error::success();
}
