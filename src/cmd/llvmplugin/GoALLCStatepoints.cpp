// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCStatepoints.h"
#include "llvm/ADT/DenseMap.h"
#include "llvm/ADT/MapVector.h"
#include "llvm/ADT/STLExtras.h"
#include "llvm/ADT/SetVector.h"
#include "llvm/ADT/SmallBitVector.h"
#include "llvm/ADT/SmallPtrSet.h"
#include "llvm/ADT/SmallSet.h"
#include "llvm/ADT/StringRef.h"
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
#include "llvm/IR/Verifier.h"
#include "llvm/Support/Alignment.h"
#include "llvm/Support/CheckedArithmetic.h"
#include "llvm/Support/Error.h"
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
  ValueSet AllocaAddresses;
  ValueSet DerivedPointers;
  CallInst *Statepoint = nullptr;
  CallInst *Result = nullptr;
  SmallVector<CallInst *, 8> Relocates;
};

struct AggregateLeaf {
  SmallVector<unsigned, 4> Indices;
};

struct PointerAllocaLeaf {
  SmallVector<unsigned, 4> Indices;
  uint64_t Offset;
  PointerType *Type;
};

struct PointerAllocaRecord {
  AllocaInst *Alloca;
  bool NeedsStackObject;
  bool DeferResult;
  bool OpenDeferSlot;
  uint64_t ByteSize;
  uint64_t Alignment;
  uint64_t BitCount;
  SmallVector<uint64_t, 4> BitmapWords;
  SmallVector<PointerAllocaLeaf, 8> Leaves;
  SmallVector<IntrinsicInst *, 4> LifetimeMarkers;
  SmallVector<Instruction *, 16> AddressUses;
  SmallVector<CallInst *, 8> ActiveCalls;
  bool ActivityUnclear = false;
};

struct PointerByValRecord {
  Argument *Base;
  bool NeedsStackObject;
  uint64_t ByteSize;
  uint64_t Alignment;
  uint64_t BitCount;
  SmallVector<uint64_t, 4> BitmapWords;
};

struct OpenDeferInfo {
  AllocaInst *Bits = nullptr;
  AllocaInst *Slots = nullptr;
  uint64_t SlotCount = 0;
};

enum class LivenessKind {
  PointerAggregates,
  RelocatablePointers,
  AllocaAddresses,
  DirectAllocaAddresses,
  DerivedPointers,
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

const AllocaInst *rematerializableAllocaBase(const Value *V) {
  while (true) {
    if (const auto *Alloca = dyn_cast<AllocaInst>(V))
      return Alloca->isStaticAlloca() ? Alloca : nullptr;
    if (const auto *GEP = dyn_cast<GetElementPtrInst>(V)) {
      V = GEP->getPointerOperand();
      continue;
    }
    if (const auto *Cast = dyn_cast<CastInst>(V);
        Cast && Cast->isNoopCast(Cast->getDataLayout())) {
      V = Cast->getOperand(0);
      continue;
    }
    return nullptr;
  }
}

AllocaInst *rematerializableAllocaBase(Value *V) {
  return const_cast<AllocaInst *>(
      rematerializableAllocaBase(static_cast<const Value *>(V)));
}

bool isStaticAllocaAddress(const Value *V) {
  if (!V->getType()->isPointerTy())
    return false;
  // Merged addresses cannot be reconstructed from one relocated alloca.
  if (isa<PHINode>(V) || isa<SelectInst>(V))
    return false;
  return rematerializableAllocaBase(V) != nullptr;
}

bool isCallTargetOnlyPointer(const Value *V) {
  if (!V->getType()->isPointerTy() || V->use_empty())
    return false;
  return llvm::all_of(V->uses(), [](const Use &U) {
    const auto *Call = dyn_cast<CallBase>(U.getUser());
    return Call && Call->isCallee(&U);
  });
}

const Value *rematerializableDerivedBase(const Value *V) {
  if (!isRelocatablePointerType(V->getType()) || isStaticAllocaAddress(V))
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

bool isDirectMemoryAddressUse(const Use &U) {
  auto *I = dyn_cast<Instruction>(U.getUser());
  if (!I)
    return false;
  if (auto *Load = dyn_cast<LoadInst>(I))
    return &U == &Load->getOperandUse(LoadInst::getPointerOperandIndex());
  if (auto *Store = dyn_cast<StoreInst>(I))
    return &U == &Store->getOperandUse(StoreInst::getPointerOperandIndex());
  if (auto *RMW = dyn_cast<AtomicRMWInst>(I))
    return &U == &RMW->getOperandUse(AtomicRMWInst::getPointerOperandIndex());
  if (auto *CmpXchg = dyn_cast<AtomicCmpXchgInst>(I))
    return &U ==
           &CmpXchg->getOperandUse(AtomicCmpXchgInst::getPointerOperandIndex());
  return false;
}

bool isDirectFrameAddressUse(const Use &U) {
  auto *I = dyn_cast<Instruction>(U.getUser());
  if (!I)
    return false;
  if (isa<GetElementPtrInst>(I) || isa<BitCastInst>(I) ||
      isa<AddrSpaceCastInst>(I) || isDirectMemoryAddressUse(U))
    return true;
  if (auto *Intrinsic = dyn_cast<IntrinsicInst>(I))
    return Intrinsic->isLifetimeStartOrEnd() ||
           Intrinsic->getIntrinsicID() == Intrinsic::fake_use ||
           isa<DbgInfoIntrinsic>(Intrinsic);
  return false;
}

Value *rematerializeAddress(Value *Address, Value *Base, Value *RelocatedBase,
                            Instruction *InsertBefore);

Error canonicalizeDirectAllocaAddresses(
    Function &F, DominatorTree &DT,
    SmallPtrSetImpl<AllocaInst *> &WholeLifetimeAllocas) {
  SmallVector<Value *, 32> Addresses;
  for (Instruction &I : instructions(F))
    if (isStaticAllocaAddress(&I))
      Addresses.push_back(&I);

  for (Value *Address : Addresses) {
    AllocaInst *Alloca = rematerializableAllocaBase(Address);
    assert(Alloca && "canonical alloca address has no static base");
    SmallVector<Use *, 8> FirstClassUses;
    SmallVector<IntrinsicInst *, 4> LifetimeStarts;
    for (Use &U : Address->uses())
      if (!isDirectFrameAddressUse(U))
        FirstClassUses.push_back(&U);
    if (FirstClassUses.empty())
      continue;

    for (Use &U : Alloca->uses())
      if (auto *II = dyn_cast<IntrinsicInst>(U.getUser());
          II && II->getIntrinsicID() == Intrinsic::lifetime_start)
        LifetimeStarts.push_back(II);

    // A Go stack can move at every ordinary call. Do not share one derived
    // frame address across calls: even though each gc.relocate is lowered back
    // to a FrameIndex, the whole-function SSA merge can be spilled by register
    // allocation and that ordinary spill slot is not a Go pointer root.
    // Rebuild the address immediately before each first-class use instead.
    DenseMap<Instruction *, Value *> UseAddresses;
    for (Use *U : FirstClassUses) {
      auto *UsePoint = cast<Instruction>(U->getUser());

      if (!LifetimeStarts.empty() &&
          !llvm::any_of(LifetimeStarts, [&](IntrinsicInst *Start) {
            // A PHI operand is used on its incoming edge, not in the PHI's
            // block. Check the concrete Use so a lifetime.start in that
            // predecessor can dominate just the alloca-carrying value.
            return DT.dominates(Start, *U);
          })) {
        // LLVM can legally move non-dereferencing address operations outside
        // the source VarDef interval. Preserve that data flow by widening the
        // storage lifetime later; Go GC activity still uses the original
        // lifetime markers as backward liveness kills.
        WholeLifetimeAllocas.insert(Alloca);
      }

      Instruction *InsertBefore = UsePoint;
      if (auto *Phi = dyn_cast<PHINode>(UsePoint))
        InsertBefore = Phi->getIncomingBlock(*U)->getTerminator();

      Value *UseAddress = UseAddresses.lookup(InsertBefore);
      if (!UseAddress) {
        if (Address == Alloca) {
          IRBuilder<> Builder(InsertBefore);
          UseAddress = Builder.CreateInBoundsGEP(
              Builder.getInt8Ty(), Alloca, Builder.getInt64(0),
              Alloca->hasName() ? Alloca->getName() + ".address"
                                : "alloca.address");
        } else {
          UseAddress =
              rematerializeAddress(Address, Alloca, Alloca, InsertBefore);
        }
        UseAddresses[InsertBefore] = UseAddress;
      }
      U->set(UseAddress);
    }
  }
  return Error::success();
}

bool isTrackedValue(const Value *V, LivenessKind Kind) {
  if (isa<Constant>(V))
    return false;
  Type *Ty = V->getType();
  switch (Kind) {
  case LivenessKind::PointerAggregates:
    return !isRelocatablePointerType(Ty) && containsPointer(Ty);
  case LivenessKind::RelocatablePointers:
    return isRelocatablePointerType(Ty) && !isStaticAllocaAddress(V) &&
           !rematerializableDerivedBase(V) && !isCallTargetOnlyPointer(V);
  case LivenessKind::AllocaAddresses:
    // Direct memory addresses must not enter relocation SSA. Under register
    // pressure, a relocated address PHI can be spilled into an ordinary
    // non-root slot; that cached address then points at the old Go stack after
    // growth. canonicalizeDirectAllocaMemoryUses rebuilds every address that
    // crosses an ordinary call at its terminal use, while
    // canonicalizeDirectAllocaAddresses does the same for first-class uses.
    // Only those first-class local values need relocation liveness here.
    return Ty->isPointerTy() && !isa<AllocaInst>(V) &&
           isStaticAllocaAddress(V) &&
           llvm::any_of(V->uses(), [](const Use &U) {
             return !isDirectFrameAddressUse(U);
           });
  case LivenessKind::DirectAllocaAddresses:
    // This temporary analysis class finds derived frame addresses whose direct
    // memory uses cross an ordinary call. Those uses are rebuilt locally
    // before statepoint rewriting and never enter relocation SSA.
    return Ty->isPointerTy() && !isa<AllocaInst>(V) &&
           isStaticAllocaAddress(V) &&
           llvm::any_of(V->uses(), isDirectMemoryAddressUse);
  case LivenessKind::DerivedPointers:
    return rematerializableDerivedBase(V) != nullptr;
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

void canonicalizeDirectAllocaMemoryUses(Function &F) {
  LivenessData Data = computeLiveness(F, LivenessKind::DirectAllocaAddresses);
  ValueSet AddressesAcrossCalls;
  for (Instruction &I : instructions(F)) {
    auto *Call = dyn_cast<CallInst>(&I);
    if (!Call || isa<GCStatepointInst>(Call) || isLeafCall(*Call))
      continue;
    AddressesAcrossCalls.set_union(
        liveAtCall(*Call, Data, LivenessKind::DirectAllocaAddresses));
  }

  for (Value *Address : AddressesAcrossCalls) {
    AllocaInst *Base = rematerializableAllocaBase(Address);
    assert(Base && "direct frame address has no static alloca base");
    SmallVector<Use *, 8> MemoryUses;
    for (Use &U : Address->uses())
      if (isDirectMemoryAddressUse(U))
        MemoryUses.push_back(&U);

    for (Use *U : MemoryUses) {
      auto *UsePoint = cast<Instruction>(U->getUser());
      Value *UseAddress = rematerializeAddress(Address, Base, Base, UsePoint);
      U->set(UseAddress);
    }
  }
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
  for (const AggregateLeaf &Leaf : Leaves) {
    // Use FindInsertedValue only to recognize leaves that do not contain a
    // defined value. Reusing an ordinary inserted SSA value would make the
    // relocation repair for this aggregate rewrite every other use of that
    // value as well; distinct aggregate leaves need distinct SSA identities.
    // Conversely, materializing an extractvalue from undef or poison would
    // invent an ordinary pointer root whose SelectionDAG spill may be removed.
    Value *Inserted = FindInsertedValue(&Aggregate, Leaf.Indices);
    Value *LeafValue =
        Inserted && isa<UndefValue, PoisonValue>(Inserted)
            ? Inserted
            : Builder.CreateExtractValue(&Aggregate, Leaf.Indices,
                                         leafName(Aggregate, Leaf.Indices));
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
          LeafInst && LeafInst->use_empty())
        LeafInst->eraseFromParent();
  }
  return Error::success();
}

Error enumeratePointerAllocaLeaves(Type *Ty, const DataLayout &DL,
                                   SmallVectorImpl<unsigned> &Path,
                                   uint64_t Offset,
                                   SmallVectorImpl<PointerAllocaLeaf> &Leaves) {
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
      if (Error Err = enumeratePointerAllocaLeaves(ElementTy, DL, Path,
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
      if (Error Err = enumeratePointerAllocaLeaves(
              AT->getElementType(), DL, Path, *ElementOffset, Leaves))
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

Value *pointerAllocaLeafAddress(IRBuilder<> &Builder, AllocaInst &Alloca,
                                const PointerAllocaLeaf &Leaf,
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

std::string allocaLeafName(AllocaInst &Alloca, const PointerAllocaLeaf &Leaf) {
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
  if (!Info.Bits || !Info.Slots)
    return createStringError(
        std::errc::invalid_argument,
        "GoALLC open-coded defer function is missing frame state metadata");

  Info.Bits->setMetadata(GoOpenDeferBitsMD, nullptr);
  Info.Slots->setMetadata(GoOpenDeferSlotsMD, nullptr);
  return std::optional<OpenDeferInfo>(std::move(Info));
}

// Return true when the optimized IR can make the address observable outside
// compiler-controlled direct accesses. This is deliberately a structural
// post-optimization decision: the frontend Addrtaken bit is provenance, not a
// promise that prevents SROA or forces a surviving frame address to remain a
// stack object.
bool addressNeedsStackObject(Value &Base) {
  SmallVector<Value *, 16> Worklist{&Base};
  SmallPtrSet<Value *, 16> Seen;
  while (!Worklist.empty()) {
    Value *Address = Worklist.pop_back_val();
    if (!Seen.insert(Address).second)
      continue;
    for (User *U : Address->users()) {
      auto *I = dyn_cast<Instruction>(U);
      if (!I)
        return true;

      if (isa<PHINode>(I) || isa<SelectInst>(I) || isa<FreezeInst>(I))
        // A merged/frozen address is tracked as an independent scalar root.
        // It needs StackObject metadata so the runtime can discover and scan
        // the alloca dynamically when that root points into this frame.
        return true;
      if (isa<GetElementPtrInst>(I) || isa<BitCastInst>(I) ||
          isa<AddrSpaceCastInst>(I)) {
        if (!I->getType()->isPointerTy())
          return true;
        Worklist.push_back(I);
        continue;
      }
      if (auto *Load = dyn_cast<LoadInst>(I)) {
        if (Load->getPointerOperand() != Address || Load->isAtomic() ||
            Load->isVolatile())
          return true;
        continue;
      }
      if (auto *Store = dyn_cast<StoreInst>(I)) {
        if (Store->getPointerOperand() != Address ||
            Store->getValueOperand() == Address || Store->isAtomic() ||
            Store->isVolatile())
          return true;
        continue;
      }
      if (isa<ICmpInst>(I))
        continue;
      if (auto *Intrinsic = dyn_cast<IntrinsicInst>(I)) {
        if (Intrinsic->isLifetimeStartOrEnd() ||
            Intrinsic->getIntrinsicID() == Intrinsic::fake_use ||
            isa<DbgInfoIntrinsic>(Intrinsic) || isa<MemIntrinsic>(Intrinsic))
          continue;
        return true;
      }
      // Passing the address to even a captures(none), readonly, or GC-leaf
      // call makes memory observable during the call. Storing/returning it,
      // ptrtoint, inline asm, and every unknown use are likewise stack-object
      // cases.
      return true;
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

bool isPointerAddressDerivation(const Instruction &I) {
  return isa<GetElementPtrInst>(I) || isa<BitCastInst>(I) ||
         isa<AddrSpaceCastInst>(I) || isa<PHINode>(I) || isa<SelectInst>(I) ||
         isa<FreezeInst>(I);
}

void collectPointerAllocaAddressUses(PointerAllocaRecord &Record) {
  SmallVector<Value *, 16> Worklist{Record.Alloca};
  SmallPtrSet<Value *, 16> SeenAddresses;
  SmallPtrSet<Instruction *, 16> SeenUses;
  while (!Worklist.empty()) {
    Value *Address = Worklist.pop_back_val();
    if (!SeenAddresses.insert(Address).second)
      continue;
    for (User *U : Address->users()) {
      auto *I = dyn_cast<Instruction>(U);
      if (!I) {
        Record.ActivityUnclear = true;
        continue;
      }
      if (isPointerAddressDerivation(*I)) {
        if (!I->getType()->isPointerTy()) {
          Record.ActivityUnclear = true;
          continue;
        }
        // Only direct GEP/cast chains remain the same frame address. A
        // phi/select/freeze result is an independent SSA pointer root: its
        // live-out does not make every possible incoming alloca live after the
        // merge.
        if (rematerializableAllocaBase(I) == Record.Alloca)
          Worklist.push_back(I);
        continue;
      }
      if (auto *Intrinsic = dyn_cast<IntrinsicInst>(I)) {
        if (Intrinsic->isLifetimeStartOrEnd() || isa<DbgInfoIntrinsic>(I))
          continue;
      }
      if (SeenUses.insert(I).second)
        Record.AddressUses.push_back(I);
    }
  }
}

bool pointerAllocaLiveInBlock(const PointerAllocaRecord &Record,
                              const BasicBlock &BB, bool LiveOut) {
  bool Live = LiveOut;
  for (const Instruction &I : llvm::reverse(BB)) {
    if (llvm::is_contained(Record.LifetimeMarkers, &I))
      Live = false;
    else if (llvm::is_contained(Record.AddressUses, &I))
      Live = true;
  }
  return Live;
}

// Go VarDef is a complete initialization boundary. Treat its lifetime.start
// as a backward kill and terminal address operations as uses. This gives each
// start a path-sensitive interval whose end is the final real use, without
// inserting lifetime.end or changing the producer's initialization sequence.
Error computePointerAllocaActivity(
    Function &F, SmallVectorImpl<PointerAllocaRecord> &PointerAllocas,
    ArrayRef<SafepointRecord> Safepoints, const DominatorTree &DT) {
  SmallPtrSet<const CallInst *, 16> SafepointCalls;
  for (const SafepointRecord &Safepoint : Safepoints)
    SafepointCalls.insert(Safepoint.Call);

  for (PointerAllocaRecord &Record : PointerAllocas) {
    collectPointerAllocaAddressUses(Record);
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
        if (Call && SafepointCalls.contains(Call) &&
            (Live || !DT.isReachableFromEntry(Call->getParent())))
          Record.ActiveCalls.push_back(Call);
        if (llvm::is_contained(Record.LifetimeMarkers, &I))
          Live = false;
        else if (llvm::is_contained(Record.AddressUses, &I))
          Live = true;
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
  for (auto [Index, Leaf] : llvm::enumerate(Record.Leaves)) {
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
  SmallBitVector Initialized(Record.Leaves.size());
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
        SmallVector<PointerAllocaLeaf, 4> StoredLeaves;
        SmallVector<unsigned, 4> Path;
        if (Error Err = enumeratePointerAllocaLeaves(StoredValue->getType(), DL,
                                                     Path, 0, StoredLeaves)) {
          consumeError(std::move(Err));
        } else {
          for (const PointerAllocaLeaf &Leaf : StoredLeaves) {
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
    if (auto *Call = dyn_cast<CallBase>(I); Call && !isLeafCall(*Call))
      return Initialized.count() == Record.Leaves.size();
  }
  return Initialized.count() == Record.Leaves.size();
}

void initializePointerAllocasForGC(
    MutableArrayRef<PointerAllocaRecord> PointerAllocas,
    const SmallPtrSetImpl<AllocaInst *> &WholeLifetimeAllocas) {
  if (PointerAllocas.empty())
    return;
  const DataLayout &DL =
      PointerAllocas.front().Alloca->getFunction()->getDataLayout();
  for (PointerAllocaRecord &Record : PointerAllocas) {
    if (WholeLifetimeAllocas.contains(Record.Alloca) || Record.DeferResult ||
        Record.OpenDeferSlot)
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
                                 Builder.getInt64(Record.ByteSize));
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

Error promoteAllocasToWholeFunctionLifetime(
    Function &F, const SmallPtrSetImpl<AllocaInst *> &WholeLifetimeAllocas) {
  if (WholeLifetimeAllocas.empty())
    return Error::success();

  SmallVector<AllocaInst *, 8> OrderedAllocas;
  SmallVector<Instruction *, 16> OldLifetimeMarkers;
  for (Instruction &I : instructions(F)) {
    if (auto *Alloca = dyn_cast<AllocaInst>(&I);
        Alloca && WholeLifetimeAllocas.contains(Alloca)) {
      OrderedAllocas.push_back(Alloca);
      continue;
    }
    auto *Lifetime = dyn_cast<LifetimeIntrinsic>(&I);
    if (!Lifetime)
      continue;
    auto *Alloca = dyn_cast<AllocaInst>(Lifetime->getArgOperand(0));
    if (Alloca && WholeLifetimeAllocas.contains(Alloca))
      OldLifetimeMarkers.push_back(Lifetime);
  }

  for (Instruction *Marker : OldLifetimeMarkers)
    Marker->eraseFromParent();

  const DataLayout &DL = F.getDataLayout();
  for (AllocaInst *Alloca : OrderedAllocas) {
    std::optional<TypeSize> AllocationSize = Alloca->getAllocationSize(DL);
    if (!AllocationSize || AllocationSize->isScalable())
      return createStringError(
          std::errc::not_supported,
          "GoALLC cannot widen a dynamically sized alloca lifetime");

    // Keep one storage lifetime from function entry through return. The
    // original VarDef markers have already served as Go GC liveness kills;
    // they must not reset this entry initialization back to uninitialized.
    IRBuilder<> Builder(Alloca->getNextNode());
    Builder.SetCurrentDebugLocation(Alloca->getDebugLoc());
    Builder.CreateLifetimeStart(Alloca);
    uint64_t ByteSize = AllocationSize->getFixedValue();
    if (ByteSize != 0)
      // GoObj has no hosted memset fallback. Keep this fixed-size entry
      // initialization inline so lowering cannot split later allocas away
      // from the entry block while expanding an unavailable libcall.
      Builder.CreateMemSetInline(Alloca, Alloca->getAlign(), Builder.getInt8(0),
                                 Builder.getInt64(ByteSize));
    Alloca->setMetadata(
        StackColoringNoMergeMD,
        MDNode::get(Alloca->getContext(), ArrayRef<Metadata *>()));
  }
  return Error::success();
}

bool isPointerAllocaActiveAt(const PointerAllocaRecord &Record,
                             const CallInst &Call) {
  return llvm::is_contained(Record.ActiveCalls, &Call);
}

Error collectPointerAllocas(
    Function &F, const std::optional<OpenDeferInfo> &OpenDefer,
    SmallVectorImpl<PointerAllocaRecord> &PointerAllocas) {
  bool HasSafepoint = llvm::any_of(instructions(F), [](Instruction &I) {
    auto *Call = dyn_cast<CallBase>(&I);
    return Call && !isa<GCStatepointInst>(Call) && !isLeafCall(*Call);
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
    std::optional<TypeSize> AllocationSize = Alloca->getAllocationSize(DL);
    if (!AllocationSize || AllocationSize->isScalable())
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints do not support scalable pointer-containing "
          "allocas");
    if (std::optional<Align> StackAlign = DL.getStackAlignment();
        StackAlign && Alloca->getAlign() > *StackAlign)
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints do not support realigned pointer-containing "
          "allocas");
    SmallVector<PointerAllocaLeaf, 8> Leaves;
    SmallVector<unsigned, 4> Path;
    if (Error Err = enumeratePointerAllocaLeaves(Alloca->getAllocatedType(), DL,
                                                 Path, 0, Leaves))
      return std::move(Err);
    uint64_t ByteSize = AllocationSize->getFixedValue();
    uint64_t PointerSize = DL.getPointerSize(0);
    if (!PointerSize || ByteSize == 0 || ByteSize % PointerSize != 0 ||
        Alloca->getAlign() < DL.getABITypeAlign(Alloca->getAllocatedType()))
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints require pointer-aligned fixed alloca layouts");
    uint64_t BitCount = ByteSize / PointerSize;
    SmallVector<uint64_t, 4> BitmapWords((BitCount + 63) / 64, 0);
    for (const PointerAllocaLeaf &Leaf : Leaves) {
      if (Leaf.Offset % PointerSize != 0 || Leaf.Offset >= ByteSize)
        return createStringError(
            std::errc::not_supported,
            "GoALLC statepoint alloca pointer slot is not pointer-aligned");
      uint64_t Bit = Leaf.Offset / PointerSize;
      uint64_t Mask = uint64_t(1) << (Bit % 64);
      if (BitmapWords[Bit / 64] & Mask)
        return createStringError(
            std::errc::invalid_argument,
            "GoALLC statepoint alloca pointer slots overlap");
      BitmapWords[Bit / 64] |= Mask;
    }
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
                              IsOpenDeferSlot, ByteSize,
                              Alloca->getAlign().value(), BitCount,
                              std::move(BitmapWords), std::move(Leaves)});
  }

  if (Error Err = collectPointerAllocaLifetimeMarkers(F, PointerAllocas))
    return Err;

  return Error::success();
}

Error collectPointerByVals(Function &F,
                           SmallVectorImpl<PointerByValRecord> &Records) {
  if (!isGoCallingConv(F.getCallingConv()))
    return Error::success();

  const DataLayout &DL = F.getDataLayout();
  uint64_t PointerSize = DL.getPointerSize(0);
  for (Argument &Arg : F.args()) {
    if (!Arg.hasByValAttr())
      continue;
    Type *StorageType = Arg.getParamByValType();
    if (!StorageType || !containsPointer(StorageType))
      continue;

    TypeSize AllocationSize = DL.getTypeAllocSize(StorageType);
    if (AllocationSize.isScalable())
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints do not support scalable byval parameter "
          "layouts");
    Align Alignment =
        Arg.getParamAlign().value_or(DL.getABITypeAlign(StorageType));
    uint64_t ByteSize = AllocationSize.getFixedValue();
    if (!PointerSize || !ByteSize || ByteSize % PointerSize != 0 ||
        Alignment < DL.getABITypeAlign(StorageType))
      return createStringError(
          std::errc::not_supported,
          "GoALLC statepoints require pointer-aligned fixed byval parameter "
          "layouts");

    SmallVector<PointerAllocaLeaf, 8> Leaves;
    SmallVector<unsigned, 4> Path;
    if (Error Err =
            enumeratePointerAllocaLeaves(StorageType, DL, Path, 0, Leaves))
      return std::move(Err);
    uint64_t BitCount = ByteSize / PointerSize;
    SmallVector<uint64_t, 4> BitmapWords((BitCount + 63) / 64, 0);
    for (const PointerAllocaLeaf &Leaf : Leaves) {
      if (Leaf.Offset % PointerSize != 0 || Leaf.Offset >= ByteSize)
        return createStringError(
            std::errc::not_supported,
            "GoALLC statepoint byval pointer slot is not pointer-aligned");
      uint64_t Bit = Leaf.Offset / PointerSize;
      uint64_t Mask = uint64_t(1) << (Bit % 64);
      if (BitmapWords[Bit / 64] & Mask)
        return createStringError(
            std::errc::invalid_argument,
            "GoALLC statepoint byval pointer slots overlap");
      BitmapWords[Bit / 64] |= Mask;
    }
    Records.push_back({&Arg, addressNeedsStackObject(Arg), ByteSize,
                       Alignment.value(), BitCount, std::move(BitmapWords)});
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
    ArrayRef<const PointerByValRecord *> ByVals,
    const SmallPtrSetImpl<Value *> &LiveContents,
    SmallVectorImpl<Value *> &Deopt) {
  if (Allocas.empty() && ByVals.empty())
    return;
  // ProtocolLength covers BEGIN through END, but not the trailing duplicate
  // length.  The envelope itself therefore contributes BEGIN, length,
  // record-count, and END.
  uint64_t ProtocolLength = 4;
  for (const PointerAllocaRecord *Alloca : Allocas)
    ProtocolLength += 11 + Alloca->BitmapWords.size();
  for (const PointerByValRecord *ByVal : ByVals)
    ProtocolLength += 11 + ByVal->BitmapWords.size();

  auto AppendConstant = [&](uint64_t Value) {
    Deopt.push_back(ConstantInt::get(Builder.getInt64Ty(), Value));
  };
  AppendConstant(GoObj::AllocaPtrMapBeginMagic);
  AppendConstant(ProtocolLength);
  AppendConstant(Allocas.size() + ByVals.size());
  auto AppendRecord = [&](Value *Base, uint64_t ByteSize, uint64_t Alignment,
                          uint64_t BitCount, ArrayRef<uint64_t> BitmapWords) {
    AppendConstant(GoObj::AllocaPtrMapRecordTag);
    AppendConstant(11 + BitmapWords.size());
    Deopt.push_back(Base);
    AppendConstant(0); // First contract version describes the whole object.
    AppendConstant(ByteSize);
    AppendConstant(Alignment);
    AppendConstant(
        Builder.GetInsertBlock()->getModule()->getDataLayout().getPointerSize(
            0));
    // gc-live also carries direct frame bases needed only to rematerialize an
    // address after stack growth. Keep object-content liveness independent so
    // GoObj never mistakes that relocate-only operand for a LocalsPointerMaps
    // root.
    AppendConstant(LiveContents.contains(Base));
    AppendConstant(BitCount);
    AppendConstant(GoObj::AllocaPtrMapBitmapWordBits);
    AppendConstant(BitmapWords.size());
    for (uint64_t Word : BitmapWords)
      AppendConstant(Word);
  };
  for (const PointerAllocaRecord *Alloca : Allocas) {
    AppendRecord(Alloca->Alloca, Alloca->ByteSize, Alloca->Alignment,
                 Alloca->BitCount, Alloca->BitmapWords);
  }
  for (const PointerByValRecord *ByVal : ByVals)
    AppendRecord(ByVal->Base, ByVal->ByteSize, ByVal->Alignment,
                 ByVal->BitCount, ByVal->BitmapWords);
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
                  ArrayRef<const PointerByValRecord *> PointerByVals,
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
  appendAllocaPtrMapDeoptOperands(Builder, PointerAllocas, PointerByVals,
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
    CallInst *Relocate = Builder.CreateGCRelocate(
        Record.Statepoint, Index, Index, V->getType(),
        V->hasName() ? V->getName() + ".relocated" : "");
    Relocate->setCallingConv(CallingConv::Cold);
    Record.Relocates.push_back(Relocate);
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

void rebuildDirectFixedFrameMemoryUsesFromRelocates(
    Function &F, DominatorTree &DT, ArrayRef<SafepointRecord> Records) {
  // canonicalizeDirectAllocaMemoryUses gives every terminal memory operation
  // its own local address expression. After the continuations have been
  // split, rebuild that expression once more from the latest dominating
  // gc.relocate of the fixed frame base. Direct uses of typed byval/goret homes
  // need the same treatment. Merely rebuilding from the original base still
  // lets SelectionDAG reuse a pre-statepoint address register; using the
  // relocate forces FrameIndexRemat in the continuation block.
  SmallVector<std::pair<Value *, Value *>, 32> Addresses;
  for (Instruction &I : instructions(F))
    if (isStaticAllocaAddress(&I))
      Addresses.push_back({&I, rematerializableAllocaBase(&I)});
  for (Argument &Arg : F.args())
    if (Arg.hasByValAttr() || Arg.hasGoRetAttr())
      Addresses.push_back({&Arg, &Arg});

  for (const auto &AddressAndBase : Addresses) {
    Value *Address = AddressAndBase.first;
    Value *Base = AddressAndBase.second;
    SmallVector<Use *, 8> MemoryUses;
    for (Use &U : Address->uses())
      if (isDirectMemoryAddressUse(U))
        MemoryUses.push_back(&U);

    for (Use *U : MemoryUses) {
      auto *UsePoint = cast<Instruction>(U->getUser());
      CallInst *RelocatedBase = nullptr;
      for (const SafepointRecord &Record : Records) {
        auto Relocate = llvm::find_if(Record.Relocates, [&](CallInst *Call) {
          return cast<GCRelocateInst>(Call)->getDerivedPtr() == Base;
        });
        if (Relocate == Record.Relocates.end() ||
            !DT.dominates(*Relocate, UsePoint))
          continue;
        if (!RelocatedBase || DT.dominates(RelocatedBase, *Relocate))
          RelocatedBase = *Relocate;
      }
      if (!RelocatedBase)
        continue;

      Value *UseAddress = Address == Base
                              ? static_cast<Value *>(RelocatedBase)
                              : rematerializeAddress(Address, Base,
                                                     RelocatedBase, UsePoint);
      U->set(UseAddress);
    }
  }
}

void repairRelocationSSA(Function &F, DominatorTree &DT,
                         ArrayRef<SafepointRecord> Records) {
  // Each ordinary relocated pointer and each rematerialized fixed-object
  // derived address is a new reaching definition of its original SSA value.
  // Static allocas and typed byval/goret arguments are themselves fixed frame
  // addresses: SelectionDAG rematerializes either from its frame index at each
  // use, so replacing the original IR value with a gc.relocate chain would
  // turn later statepoints into ordinary pointer spills.
  MapVector<Value *, SmallVector<Value *, 4>> Definitions;
  for (const SafepointRecord &Record : Records) {
    for (CallInst *RelocateCall : Record.Relocates) {
      auto *Relocate = cast<GCRelocateInst>(RelocateCall);
      Value *Original = Relocate->getDerivedPtr();
      auto *Arg = dyn_cast<Argument>(Original);
      if (!isa<AllocaInst>(Original) &&
          !(Arg && (Arg->hasByValAttr() || Arg->hasGoRetAttr())))
        Definitions[Original].push_back(RelocateCall);
    }

    Instruction *InsertBefore = Record.Relocates.empty()
                                    ? Record.Statepoint->getNextNode()
                                    : Record.Relocates.back()->getNextNode();
    for (Value *Address : Record.AllocaAddresses) {
      AllocaInst *Base = rematerializableAllocaBase(Address);
      auto Relocate = llvm::find_if(Record.Relocates, [&](CallInst *Call) {
        return cast<GCRelocateInst>(Call)->getDerivedPtr() == Base;
      });
      assert(Relocate != Record.Relocates.end() &&
             "alloca address is missing its base relocate");
      Value *Rematerialized =
          rematerializeAddress(Address, Base, *Relocate, InsertBefore);
      Definitions[Address].push_back(Rematerialized);
    }
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
      if (isa<GCStatepointInst>(Call) || !isLeafCall(*Call))
        return createStringError(
            std::errc::invalid_argument,
            "GoALLC gc-leaf-function contains a non-leaf call");
    }
    return Error::success();
  }

  DominatorTree DT(F);
  SmallPtrSet<AllocaInst *, 8> WholeLifetimeAllocas;
  if (Error Err =
          canonicalizeDirectAllocaAddresses(F, DT, WholeLifetimeAllocas))
    return Err;
  // Static alloca-derived values that reach only load/store/atomic pointer
  // operands intentionally stay out of relocation SSA. Rebuild every such
  // terminal use when its address is live across an ordinary call. After all
  // calls become statepoints, splitStatepointContinuations places each rebuilt
  // use in the continuation block, preventing SelectionDAG from carrying its
  // pre-growth address across the statepoint.
  canonicalizeDirectAllocaMemoryUses(F);
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
  SmallVector<PointerAllocaRecord, 8> PointerAllocas;
  if (Error Err = collectPointerAllocas(F, OpenDefer, PointerAllocas))
    return Err;
  SmallVector<PointerByValRecord, 4> PointerByVals;
  if (Error Err = collectPointerByVals(F, PointerByVals))
    return Err;
  if (Error Err = scalarizeLivePointerAggregates(F))
    return Err;

  LivenessData Data = computeLiveness(F, LivenessKind::RelocatablePointers);
  LivenessData AddressData = computeLiveness(F, LivenessKind::AllocaAddresses);
  LivenessData DerivedData = computeLiveness(F, LivenessKind::DerivedPointers);
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
         liveAtCall(*OrdinaryCall, Data, LivenessKind::RelocatablePointers),
         liveAtCall(*OrdinaryCall, AddressData, LivenessKind::AllocaAddresses),
         liveAtCall(*OrdinaryCall, DerivedData,
                    LivenessKind::DerivedPointers)});
  }
  for (SafepointRecord &Record : Records)
    for (Value *Address : Record.DerivedPointers) {
      // A hoisted GEP from null can be a small non-Go pointer. Keep only its
      // relocatable base in Go's stack map; repairRelocationSSA reconstructs
      // the exact address expression after the statepoint.
      Record.Live.insert(rematerializableDerivedBase(Address));
    }
  for (const SafepointRecord &Record : Records)
    if (Error Err = validateSafepoint(Record))
      return Err;
  if (Error Err = computePointerAllocaActivity(F, PointerAllocas, Records, DT))
    return Err;
  DenseMap<AllocaInst *, const PointerAllocaRecord *> PointerAllocaRecords;
  for (const PointerAllocaRecord &Alloca : PointerAllocas)
    PointerAllocaRecords[Alloca.Alloca] = &Alloca;

  for (SafepointRecord &Record : Records) {
    SmallVector<Value *, 8> InactiveAddresses;
    for (Value *Address : Record.AllocaAddresses) {
      AllocaInst *Base = rematerializableAllocaBase(Address);
      bool IsActive = true;
      if (PointerAllocaRecords.contains(Base)) {
        // Record.AllocaAddresses is already the standard SSA live-out set for
        // this call. Do not replace it with the storage activity approximation.
      } else {
        SmallVector<IntrinsicInst *, 4> LifetimeStarts;
        for (User *U : Base->users())
          if (auto *II = dyn_cast<IntrinsicInst>(U);
              II && II->getIntrinsicID() == Intrinsic::lifetime_start)
            LifetimeStarts.push_back(II);
        if (!LifetimeStarts.empty())
          IsActive = llvm::any_of(LifetimeStarts, [&](IntrinsicInst *Start) {
            return DT.dominates(Start, Record.Call);
          });
      }
      if (IsActive)
        Record.Live.insert(Base);
      else
        InactiveAddresses.push_back(Address);
    }
    for (Value *Address : InactiveAddresses)
      Record.AllocaAddresses.remove(Address);
  }
  protectStackObjectsFromColoring(PointerAllocas);
  initializePointerAllocasForGC(PointerAllocas, WholeLifetimeAllocas);
  if (Error Err =
          promoteAllocasToWholeFunctionLifetime(F, WholeLifetimeAllocas))
    return Err;
  for (SafepointRecord &Record : llvm::reverse(Records)) {
    SmallVector<const PointerAllocaRecord *, 8> AllocaRecords;
    SmallPtrSet<Value *, 8> LiveContents;
    for (const PointerAllocaRecord &Alloca : PointerAllocas) {
      // A recovered panic resumes outside LLVM's explicit CFG. The frontend
      // marks named result homes whose contents must therefore remain visible
      // to Go's stack scanner at every possible suspension call.
      bool ContentsLive = Alloca.DeferResult || Alloca.OpenDeferSlot ||
                          isPointerAllocaActiveAt(Alloca, *Record.Call);
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
    SmallVector<const PointerByValRecord *, 4> ByValRecords;
    for (const PointerByValRecord &ByVal : PointerByVals) {
      // A typed byval parameter is a caller-initialized fixed Go argument
      // object. Standard SSA liveness decides when its contents contribute to
      // this call's ArgsPointerMaps. If its address is observable, carry the
      // layout at every call so GoObj can also infer the function-level
      // StackObject from an inactive contents record.
      bool IsActive = Record.Live.contains(ByVal.Base);
      if (IsActive) {
        LiveContents.insert(ByVal.Base);
      }
      if (IsActive || ByVal.NeedsStackObject)
        ByValRecords.push_back(&ByVal);
    }
    if (Error Err = rewriteCall(Record, AllocaRecords, ByValRecords,
                                LiveContents, OpenDefer))
      return Err;
  }
  eraseOriginalCalls(Records);
  splitStatepointContinuations(Records);
  // splitStatepointContinuations changes the CFG after liveness and
  // object-activity analysis.
  // The direct-memory and general relocation repairs use this tree, and the
  // latter promotes temporary merge slots, so rebuild it for the new
  // continuation blocks.
  DT.recalculate(F);
  rebuildDirectFixedFrameMemoryUsesFromRelocates(F, DT, Records);
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

Error lowerPointerAddressObservations(Module &M) {
  SmallVector<CallInst *, 8> Calls;
  SmallVector<Function *, 2> Declarations;
  for (Function &F : M) {
    if (F.getIntrinsicID() != Intrinsic::go_pointer_address)
      continue;
    Declarations.push_back(&F);
    for (User *U : F.users()) {
      auto *Call = dyn_cast<CallInst>(U);
      if (!Call || Call->getCalledFunction() != &F)
        return createStringError(std::errc::invalid_argument,
                                 "llvm.go.pointer.address has an invalid use");
      Calls.push_back(Call);
    }
  }

  for (CallInst *Call : Calls) {
    IRBuilder<> Builder(Call);
    Builder.SetCurrentDebugLocation(Call->getDebugLoc());
    Value *Address = Builder.CreatePtrToInt(
        Call->getArgOperand(0), Call->getType(), Call->getName() + ".lowered");
    Call->replaceAllUsesWith(Address);
    Call->eraseFromParent();
  }
  for (Function *Declaration : Declarations)
    if (Declaration->use_empty())
      Declaration->eraseFromParent();
  return Error::success();
}

} // namespace

Error goallc::prepareStatepointModule(Module &M) {
  if (Error Err = lowerPointerAddressObservations(M))
    return Err;
  if (Error Err = materializeFunctionMarkerRelocs(M))
    return Err;
  return Error::success();
}

Error goallc::rewriteStatepoints(Function &F, TargetMachine &) {
  if (F.isDeclaration() || !isGoCallingConv(F.getCallingConv()) || !F.hasGC() ||
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
