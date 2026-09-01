// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCCPUFeatures.h"

#include "llvm/ADT/ArrayRef.h"
#include "llvm/ADT/STLExtras.h"
#include "llvm/ADT/SmallVector.h"
#include "llvm/ADT/StringMap.h"
#include "llvm/ADT/StringRef.h"
#include "llvm/IR/Attributes.h"
#include "llvm/IR/Constants.h"
#include "llvm/IR/DebugInfoMetadata.h"
#include "llvm/IR/Function.h"
#include "llvm/IR/GlobalVariable.h"
#include "llvm/IR/IRBuilder.h"
#include "llvm/IR/Instructions.h"
#include "llvm/IR/Metadata.h"
#include "llvm/IR/Module.h"
#include "llvm/IR/TrackingMDRef.h"
#include "llvm/IR/Verifier.h"
#include "llvm/Support/Alignment.h"
#include "llvm/Support/Error.h"
#include "llvm/Transforms/Utils/Cloning.h"
#include "llvm/Transforms/Utils/Local.h"
#include "llvm/Transforms/Utils/ValueMapper.h"

#include <cstdint>
#include <string>

using namespace llvm;

namespace {

constexpr StringLiteral ConfigMD = "goallc.cpu.config";
constexpr StringLiteral DoneMD = "goallc.cpu.fmv.done";
constexpr StringLiteral GuardMD = "goallc.cpu.guard";
constexpr StringLiteral RequiresMD = "goallc.cpu.requires";
constexpr StringLiteral MultiversionAttr = "goallc.cpu.multiversion";
constexpr StringLiteral RuntimeFeatureMask = "runtime.goallcCPUFeatures";
constexpr StringLiteral GoResultsTupleAttr = "go_results_tuple";
constexpr StringLiteral GoObjDebugFuncsMD = "goobj.debug.funcs";
constexpr StringLiteral GoObjDebugInlineRequiredMD =
    "goobj.debug.inline.required";
constexpr StringLiteral GoObjNonPackageMD = "goobj.symbol.nonpackage";
constexpr StringLiteral GoObjSymbolFlagsMD = "goobj.symbol.flags";
constexpr StringLiteral GoObjFuncInfoMD = "goobj.func.info";
constexpr StringLiteral GoNoSplitAttr = "go-nosplit";
constexpr StringLiteral TailTransferAttr = "goallc.cpu.tail-transfers";
constexpr uint64_t GoObjSymFlagDupok = uint64_t{1} << 0;

enum FeatureBit : uint64_t {
#define GOALLC_CPU_FEATURE(Name, Bit) Feature##Name = uint64_t{1} << Bit,
#include "GoALLCCPUFeatures.def"
#undef GOALLC_CPU_FEATURE
};

struct Profile {
  StringLiteral Name;
  StringLiteral Suffix;
  StringLiteral TargetFeature;
  StringLiteral Arch;
  uint64_t Closure;
};

struct CPUConfig {
  StringRef Arch;
  uint64_t Baseline;
};

// Profiles describe Go's effective feature booleans, not a CPUID implication
// graph. internal/cpu already folds the hardware and OS requirements into
// HasFMA, while GODEBUG may independently disable sse41, avx, or fma. Treating
// one enabled boolean as another would therefore change Go program semantics.
constexpr uint64_t SSE41Closure = FeatureSSE41;
constexpr uint64_t FMAClosure = FeatureFMA;
constexpr uint64_t POPCNTClosure = FeaturePOPCNT;
constexpr uint64_t ARM64LSEClosure = FeatureARM64LSE;

constexpr uint64_t V2Baseline =
    FeatureSSE3 | FeatureSSSE3 | FeatureSSE41 | FeatureSSE42 | FeaturePOPCNT;
constexpr uint64_t V3Baseline = V2Baseline | FeatureAVX | FeatureFMA;

constexpr Profile SSE41Profile = {"x86.sse41", "sse41", "+sse4.1", "amd64",
                                  SSE41Closure};
constexpr Profile FMAProfile = {"x86.fma", "fma", "+fma", "amd64", FMAClosure};
constexpr Profile POPCNTProfile = {"x86.popcnt", "popcnt", "+popcnt", "amd64",
                                   POPCNTClosure};
constexpr Profile ARM64LSEProfile = {"arm64.lse", "lse", "+lse", "arm64",
                                     ARM64LSEClosure};

const Profile *findProfile(StringRef Name) {
  if (Name == FMAProfile.Name)
    return &FMAProfile;
  if (Name == SSE41Profile.Name)
    return &SSE41Profile;
  if (Name == POPCNTProfile.Name)
    return &POPCNTProfile;
  if (Name == ARM64LSEProfile.Name)
    return &ARM64LSEProfile;
  return nullptr;
}

Expected<StringRef> getMetadataString(const MDNode &Node, StringRef Context,
                                      unsigned Index) {
  if (Index >= Node.getNumOperands())
    return createStringError(inconvertibleErrorCode(),
                             Context + " is missing an operand");
  auto *Text = dyn_cast_or_null<MDString>(Node.getOperand(Index));
  if (!Text)
    return createStringError(inconvertibleErrorCode(),
                             Context + " operands must be strings");
  return Text->getString();
}

Expected<StringRef> getInstructionProfile(const Instruction &I,
                                          StringRef Kind) {
  const MDNode *Node = I.getMetadata(Kind);
  if (!Node || Node->getNumOperands() != 1)
    return createStringError(inconvertibleErrorCode(),
                             "!" + Kind + " must contain one profile string");
  return getMetadataString(*Node, Kind, 0);
}

Expected<CPUConfig> getCPUConfig(const Module &M) {
  const NamedMDNode *Config = M.getNamedMetadata(ConfigMD);
  if (!Config || Config->getNumOperands() != 1)
    return createStringError(inconvertibleErrorCode(),
                             "!goallc.cpu.config must contain one entry");
  const MDNode *Entry = Config->getOperand(0);
  if (!Entry || Entry->getNumOperands() != 3)
    return createStringError(
        inconvertibleErrorCode(),
        "!goallc.cpu.config entry must contain version, arch, and level");
  Expected<StringRef> Version = getMetadataString(*Entry, ConfigMD, 0);
  if (!Version)
    return Version.takeError();
  Expected<StringRef> Arch = getMetadataString(*Entry, ConfigMD, 1);
  if (!Arch)
    return Arch.takeError();
  Expected<StringRef> Level = getMetadataString(*Entry, ConfigMD, 2);
  if (!Level)
    return Level.takeError();
  if (*Version != "goallc.cpu.v1")
    return createStringError(inconvertibleErrorCode(),
                             "unsupported GoALLC CPU config version " +
                                 *Version);
  if (*Arch == "amd64") {
    if (*Level == "v1")
      return CPUConfig{*Arch, uint64_t{0}};
    if (*Level == "v2")
      return CPUConfig{*Arch, V2Baseline};
    if (*Level == "v3" || *Level == "v4")
      return CPUConfig{*Arch, V3Baseline};
    return createStringError(inconvertibleErrorCode(),
                             "unsupported GOAMD64 level " + *Level);
  }
  if (*Arch == "arm64") {
    SmallVector<StringRef, 4> Parts;
    Level->split(Parts, ',', -1, false);
    if (Parts.empty() ||
        !(Parts.front().starts_with("v8.") || Parts.front().starts_with("v9.")))
      return createStringError(inconvertibleErrorCode(),
                               "unsupported GOARM64 level " + *Level);
    uint64_t Baseline = 0;
    if (llvm::is_contained(Parts, StringRef("lse")))
      Baseline |= FeatureARM64LSE;
    return CPUConfig{*Arch, Baseline};
  }
  return CPUConfig{*Arch, uint64_t{0}};
}

void markGoObjNonPackage(GlobalObject &GO) {
  Metadata *Operands[] = {
      ConstantAsMetadata::get(ConstantInt::getTrue(GO.getContext()))};
  GO.setMetadata(GoObjNonPackageMD, MDNode::get(GO.getContext(), Operands));
}

bool isGoObjDuplicateOK(const GlobalObject &GO) {
  if (GO.isWeakForLinker())
    return true;
  const MDNode *Flags = GO.getMetadata(GoObjSymbolFlagsMD);
  if (!Flags || Flags->getNumOperands() != 2)
    return false;
  const auto *Flag = mdconst::dyn_extract<ConstantInt>(Flags->getOperand(0));
  return Flag && (Flag->getZExtValue() & GoObjSymFlagDupok) != 0;
}

void markGoObjDuplicateOK(GlobalObject &GO) {
  Metadata *Operands[] = {
      ConstantAsMetadata::get(ConstantInt::get(
          Type::getInt32Ty(GO.getContext()), GoObjSymFlagDupok)),
      ConstantAsMetadata::get(
          ConstantInt::get(Type::getInt32Ty(GO.getContext()), 0))};
  GO.setMetadata(GoObjSymbolFlagsMD, MDNode::get(GO.getContext(), Operands));
}

void eraseGoObjSourceSymbolIdentity(Function &F) {
  // Variants are internal implementation symbols, not additional Go source
  // definitions. Preserve code-generation attributes, instruction metadata,
  // and FuncInfo, but do not duplicate package-level symbol identity such as
  // PkgInit, Linkname, or ReflectMethod.
  for (StringRef Name :
       {"goobj.symbol.index", "goobj.symbol.name", "goobj.symbol.flags",
        "goobj.import", "goobj.builtin"})
    F.setMetadata(Name, nullptr);
  markGoObjNonPackage(F);
}

void eraseFunctionBodyPreservingMetadata(Function &F) {
  // Function::deleteBody clears all function metadata, including the package
  // symbol index used by cross-package GoObj references. Remove only the old
  // basic blocks so the dispatcher retains the source definition's identity.
  for (BasicBlock &BB : F)
    BB.dropAllReferences();
  while (!F.empty())
    F.begin()->eraseFromParent();
}

Error cloneRequiredInlineLocations(Function &Source, Function &Clone,
                                   ValueToValueMapTy &VMap) {
  NamedMDNode *Locations =
      Source.getParent()->getNamedMetadata(GoObjDebugInlineRequiredMD);
  if (!Locations)
    return Error::success();

  SmallVector<DILocation *, 16> MappedLocations;
  for (const MDNode *Entry : Locations->operands()) {
    if (Entry->getNumOperands() != 2)
      return createStringError(
          inconvertibleErrorCode(),
          "expected !goobj.debug.inline.required entries to have two operands");
    const auto *CAM =
        dyn_cast_or_null<ConstantAsMetadata>(Entry->getOperand(0));
    const auto *GV = CAM ? dyn_cast<GlobalValue>(CAM->getValue()) : nullptr;
    const auto *Loc = dyn_cast_or_null<DILocation>(Entry->getOperand(1));
    if (!GV || !Loc || !Loc->getInlinedAt())
      return createStringError(inconvertibleErrorCode(),
                               "invalid !goobj.debug.inline.required entry");
    if (GV != &Source)
      continue;

    // Match CloneFunction's same-module debug mapping policy. In particular,
    // keep subprograms and lexical scopes belonging to an inlined callee by
    // identity. Required locations can describe an edge whose last real
    // instruction has already disappeared, so those scopes are not
    // necessarily present in VMap even though CloneFunction normally keeps
    // them. Cloning such a scope creates a second DISubprogram with no entry
    // in !goobj.debug.funcs, leaving GoObj unable to resolve its exact symbol.
    DISubprogram *SourceSP = Source.getSubprogram();
    MetadataPredicate IdentityMD = [SourceSP](const Metadata *MD) {
      if (isa<DICompileUnit>(MD))
        return true;
      if (const auto *Scope = dyn_cast<DILocalScope>(MD))
        return Scope->getSubprogram() != SourceSP;
      return false;
    };
    auto *Mapped = dyn_cast_or_null<DILocation>(
        MapMetadata(Loc, VMap, RF_None, nullptr, nullptr, &IdentityMD));
    if (!Mapped || !Mapped->getInlinedAt())
      return createStringError(inconvertibleErrorCode(),
                               "failed to map required inline location from " +
                                   Source.getName() + " to " + Clone.getName());
    MappedLocations.push_back(Mapped);
  }

  for (DILocation *Loc : MappedLocations) {
    Metadata *Operands[] = {ConstantAsMetadata::get(&Clone), Loc};
    Locations->addOperand(MDNode::get(Source.getContext(), Operands));
  }
  return Error::success();
}

Error eraseRequiredInlineLocations(Function &F) {
  NamedMDNode *Locations =
      F.getParent()->getNamedMetadata(GoObjDebugInlineRequiredMD);
  if (!Locations)
    return Error::success();

  SmallVector<TrackingMDNodeRef, 16> Kept;
  for (MDNode *Entry : Locations->operands()) {
    if (Entry->getNumOperands() != 2)
      return createStringError(
          inconvertibleErrorCode(),
          "expected !goobj.debug.inline.required entries to have two operands");
    const auto *CAM =
        dyn_cast_or_null<ConstantAsMetadata>(Entry->getOperand(0));
    const auto *GV = CAM ? dyn_cast<GlobalValue>(CAM->getValue()) : nullptr;
    const auto *Loc = dyn_cast_or_null<DILocation>(Entry->getOperand(1));
    if (!GV || !Loc || !Loc->getInlinedAt())
      return createStringError(inconvertibleErrorCode(),
                               "invalid !goobj.debug.inline.required entry");
    if (GV != &F)
      Kept.emplace_back(Entry);
  }

  Locations->clearOperands();
  for (const TrackingMDNodeRef &Entry : Kept)
    Locations->addOperand(Entry.get());
  return Error::success();
}

void makeFramelessDispatcher(Function &F) {
  F.addFnAttr(GoNoSplitAttr);
  F.addFnAttr(TailTransferAttr);
}

void makeFramelessResolver(Function &F) {
  F.removeFnAttr(Attribute::AlwaysInline);
  F.addFnAttr(Attribute::NoInline);
  F.addFnAttr(GoNoSplitAttr);
}

void setSyntheticDebugLocation(IRBuilder<> &B, Function &F) {
  if (DISubprogram *SP = F.getSubprogram())
    B.SetCurrentDebugLocation(
        DILocation::get(F.getContext(), SP->getScopeLine(), 0, SP));
}

void addTargetFeature(Function &F, StringRef Feature) {
  std::string Features;
  Attribute Existing = F.getFnAttribute("target-features");
  if (Existing.isStringAttribute())
    Features = Existing.getValueAsString().str();
  if (!Features.empty())
    Features += ',';
  Features += Feature;
  F.addFnAttr("target-features", Features);
}

Expected<bool> specializeGuards(Function &F, uint64_t Available) {
  SmallVector<LoadInst *, 4> Guards;
  for (BasicBlock &BB : F)
    for (Instruction &I : BB)
      if (auto *Load = dyn_cast<LoadInst>(&I);
          Load && Load->getMetadata(GuardMD))
        Guards.push_back(Load);

  for (LoadInst *Load : Guards) {
    Expected<StringRef> Name = getInstructionProfile(*Load, GuardMD);
    if (!Name)
      return Name.takeError();
    const Profile *P = findProfile(*Name);
    if (!P)
      return createStringError(inconvertibleErrorCode(),
                               "unknown GoALLC CPU guard profile " + *Name);
    bool Enabled = (Available & P->Closure) == P->Closure;
    Load->replaceAllUsesWith(ConstantInt::get(Load->getType(), Enabled));
    Load->eraseFromParent();
  }

  // Only the guard-specialized constant path needs folding here. Keep this
  // local and analysis-free: the enclosing module pass runs before the normal
  // PassBuilder pipeline has established cross-analysis proxies.
  for (BasicBlock &BB : F)
    SimplifyInstructionsInBlock(&BB);
  for (BasicBlock &BB : F)
    ConstantFoldTerminator(&BB, true);
  removeUnreachableBlocks(F);
  for (BasicBlock &BB : F)
    SimplifyInstructionsInBlock(&BB);
  return !Guards.empty();
}

Error verifyRequirements(Function &F, uint64_t Available) {
  for (BasicBlock &BB : F) {
    for (Instruction &I : BB) {
      if (!I.getMetadata(RequiresMD))
        continue;
      Expected<StringRef> Name = getInstructionProfile(I, RequiresMD);
      if (!Name)
        return Name.takeError();
      const Profile *P = findProfile(*Name);
      if (!P)
        return createStringError(inconvertibleErrorCode(),
                                 "unknown GoALLC CPU requirement profile " +
                                     *Name);
      if ((Available & P->Closure) != P->Closure)
        return createStringError(inconvertibleErrorCode(),
                                 "GoALLC CPU requirement " + *Name +
                                     " survives in function " + F.getName() +
                                     " without the required target features");
    }
  }
  return Error::success();
}

AttributeList callAttributes(const Function &F) {
  AttributeList Source = F.getAttributes();
  AttrBuilder FnAttrs(F.getContext());
  if (F.hasFnAttribute(GoResultsTupleAttr))
    FnAttrs.addAttribute(GoResultsTupleAttr);
  SmallVector<AttributeSet, 8> Params;
  for (const Argument &Arg : F.args())
    Params.push_back(Source.getParamAttrs(Arg.getArgNo()));
  return AttributeList::get(F.getContext(),
                            AttributeSet::get(F.getContext(), FnAttrs),
                            Source.getRetAttrs(), Params);
}

CallInst *createTailCall(IRBuilder<> &B, Function &Signature, Value *Callee,
                         CallInst::TailCallKind Kind) {
  SmallVector<Value *, 8> Args;
  for (Argument &Arg : Signature.args())
    Args.push_back(&Arg);
  CallInst *Call = B.CreateCall(Signature.getFunctionType(), Callee, Args);
  Call->setCallingConv(Signature.getCallingConv());
  Call->setAttributes(callAttributes(Signature));
  Call->setTailCallKind(Kind);
  return Call;
}

void createTailReturn(IRBuilder<> &B, Function &Signature, Value *Callee,
                      CallInst::TailCallKind Kind = CallInst::TCK_Tail) {
  CallInst *Call = createTailCall(B, Signature, Callee, Kind);
  if (Signature.getReturnType()->isVoidTy())
    B.CreateRetVoid();
  else
    B.CreateRet(Call);
}

struct Variant {
  Function *F;
  uint64_t RuntimeRequired;
};

Expected<SmallVector<const Profile *, 4>> requestedProfiles(Function &F,
                                                            StringRef Arch) {
  Attribute Attr = F.getFnAttribute(MultiversionAttr);
  if (!Attr.isStringAttribute() || Attr.getValueAsString().empty())
    return createStringError(
        inconvertibleErrorCode(),
        "GoALLC multiversion function has an empty profile list");
  SmallVector<const Profile *, 4> Result;
  StringMap<bool> Seen;
  SmallVector<StringRef, 2> Names;
  Attr.getValueAsString().split(Names, ',', -1, false);
  for (StringRef Name : Names) {
    const Profile *P = findProfile(Name);
    if (!P)
      return createStringError(inconvertibleErrorCode(),
                               "unknown GoALLC CPU multiversion profile " +
                                   Name);
    if (P->Arch != Arch)
      return createStringError(inconvertibleErrorCode(),
                               "GoALLC CPU profile " + Name +
                                   " does not match module architecture " +
                                   Arch);
    if (!Seen.insert({Name, true}).second)
      return createStringError(inconvertibleErrorCode(),
                               "duplicate GoALLC CPU multiversion profile " +
                                   Name);
    Result.push_back(P);
  }
  return Result;
}

Expected<Function *> cloneVariant(Function &Source, StringRef Suffix,
                                  uint64_t Available,
                                  ArrayRef<const Profile *> EnabledProfiles) {
  const bool DuplicateOK = isGoObjDuplicateOK(Source);
  ValueToValueMapTy VMap;
  Function *Clone = CloneFunction(&Source, VMap);
  Clone->setName(Source.getName() + ".goallc.fmv." + Suffix);
  Clone->setLinkage(GlobalValue::InternalLinkage);
  Clone->setDSOLocal(true);
  Clone->removeFnAttr(MultiversionAttr);
  // This clone is the source function's physical execution frame. Retain the
  // complete FuncInfo node so every FuncID and FuncFlag keeps its runtime stack
  // semantics (gopanic, Wrapper, TopFrame, SPWrite, and future additions).
  eraseGoObjSourceSymbolIdentity(*Clone);
  if (DuplicateOK)
    markGoObjDuplicateOK(*Clone);
  if (Error Err = cloneRequiredInlineLocations(Source, *Clone, VMap))
    return std::move(Err);
  for (const Profile *P : EnabledProfiles)
    addTargetFeature(*Clone, P->TargetFeature);
  Expected<bool> Specialized = specializeGuards(*Clone, Available);
  if (!Specialized)
    return Specialized.takeError();
  return Clone;
}

Error registerGoObjDebugFunction(Function &F) {
  NamedMDNode *Funcs = F.getParent()->getNamedMetadata(GoObjDebugFuncsMD);
  if (!Funcs)
    return Error::success();
  DISubprogram *SP = F.getSubprogram();
  if (!SP)
    return createStringError(inconvertibleErrorCode(),
                             "GoALLC CPU variant " + F.getName() +
                                 " has no debug subprogram");
  Metadata *Operands[] = {SP, ConstantAsMetadata::get(&F)};
  Funcs->addOperand(MDNode::get(F.getContext(), Operands));
  return Error::success();
}

Function *cloneResolver(Function &Source) {
  const bool DuplicateOK = isGoObjDuplicateOK(Source);
  ValueToValueMapTy VMap;
  Function *Resolver = CloneFunction(&Source, VMap);
  Resolver->setName(Source.getName() + ".goallc.fmv.resolve");
  Resolver->setLinkage(GlobalValue::InternalLinkage);
  Resolver->setDSOLocal(true);
  Resolver->removeFnAttr(MultiversionAttr);
  eraseGoObjSourceSymbolIdentity(*Resolver);
  // The resolver only selects and tail-transfers to an executable variant; it
  // is not another semantic instance of the source function.
  Resolver->setMetadata(GoObjFuncInfoMD, nullptr);
  if (DuplicateOK)
    markGoObjDuplicateOK(*Resolver);
  eraseFunctionBodyPreservingMetadata(*Resolver);
  makeFramelessResolver(*Resolver);
  return Resolver;
}

Error multiversionFunction(Function &F, const CPUConfig &Config,
                           GlobalVariable &RuntimeMask) {
  const bool DuplicateOK = isGoObjDuplicateOK(F);
  Expected<SmallVector<const Profile *, 4>> Requested =
      requestedProfiles(F, Config.Arch);
  if (!Requested)
    return Requested.takeError();

  Expected<Function *> BaselineOrErr =
      cloneVariant(F, "baseline", Config.Baseline, {});
  if (!BaselineOrErr)
    return BaselineOrErr.takeError();
  Function *BaselineImpl = *BaselineOrErr;
  if (Error Err = verifyRequirements(*BaselineImpl, Config.Baseline))
    return Err;
  if (Error Err = registerGoObjDebugFunction(*BaselineImpl))
    return Err;

  SmallVector<const Profile *, 4> OrderedProfiles;
  for (const Profile *P :
       {&SSE41Profile, &FMAProfile, &POPCNTProfile, &ARM64LSEProfile}) {
    if (llvm::find(*Requested, P) != Requested->end())
      OrderedProfiles.push_back(P);
  }

  SmallVector<Variant, 3> Variants;
  // Generate the non-empty profile subsets in fixed order. The combination
  // clone matters when one Go function contains independently controlled
  // guards: selecting an FMA clone must not silently force its SSE4.1 guard on,
  // yet a machine with both effective features should still get both paths.
  const unsigned SubsetEnd = 1U << OrderedProfiles.size();
  for (unsigned Subset = 1; Subset < SubsetEnd; ++Subset) {
    SmallVector<const Profile *, 2> EnabledProfiles;
    std::string Suffix;
    uint64_t Required = 0;
    for (unsigned I = 0; I < OrderedProfiles.size(); ++I) {
      if (!(Subset & (1U << I)))
        continue;
      const Profile *P = OrderedProfiles[I];
      EnabledProfiles.push_back(P);
      if (!Suffix.empty())
        Suffix += '-';
      Suffix += P->Suffix;
      Required |= P->Closure;
    }
    uint64_t Available = Config.Baseline | Required;
    Expected<Function *> CloneOrErr =
        cloneVariant(F, Suffix, Available, EnabledProfiles);
    if (!CloneOrErr)
      return CloneOrErr.takeError();
    Function *Clone = *CloneOrErr;
    if (Error Err = verifyRequirements(*Clone, Available))
      return Err;
    if (Error Err = registerGoObjDebugFunction(*Clone))
      return Err;
    Variants.push_back({Clone, Required & ~Config.Baseline});
  }

  LLVMContext &C = F.getContext();
  Module &M = *F.getParent();
  Function *Resolver = cloneResolver(F);
  if (Error Err = registerGoObjDebugFunction(*Resolver))
    return Err;

  auto *Slot = new GlobalVariable(M, PointerType::getUnqual(C), false,
                                  GlobalValue::InternalLinkage, Resolver,
                                  F.getName() + ".goallc.fmv.slot");
  Slot->setAlignment(Align(M.getDataLayout().getPointerABIAlignment(0)));
  Slot->setDSOLocal(true);
  Slot->setSection(".noptrdata");
  markGoObjNonPackage(*Slot);
  if (DuplicateOK)
    markGoObjDuplicateOK(*Slot);

  if (Error Err = eraseRequiredInlineLocations(F))
    return Err;
  eraseFunctionBodyPreservingMetadata(F);
  F.removeFnAttr(MultiversionAttr);
  // This synthetic dispatcher has no stack frame. Its terminal tail transfer
  // is not a safepoint, while normal optimization may inline it into a
  // same-module caller where a non-terminal call is rewritten as usual.
  makeFramelessDispatcher(F);
  BasicBlock *Entry = BasicBlock::Create(C, "entry", &F);
  IRBuilder<> B(Entry);
  // Keep the synthetic load and tail transfer locationless. If this tiny
  // dispatcher is inlined, attributing those instructions to the source
  // function would add a second logical frame beside the physical variant.
  LoadInst *Target = B.CreateLoad(PointerType::getUnqual(C), Slot, "target");
  Target->setAtomic(AtomicOrdering::Monotonic);
  Target->setAlignment(Slot->getAlign().valueOrOne());
  createTailReturn(B, F, Target);

  BasicBlock *ResolveEntry = BasicBlock::Create(C, "entry", Resolver);
  BasicBlock *Uninitialized = BasicBlock::Create(C, "uninitialized", Resolver);
  BasicBlock *Select = BasicBlock::Create(C, "select", Resolver);
  B.SetInsertPoint(ResolveEntry);
  setSyntheticDebugLocation(B, *Resolver);
  LoadInst *Mask = B.CreateLoad(Type::getInt64Ty(C), &RuntimeMask, "features");
  Mask->setAlignment(Align(8));
  Value *Initialized = B.CreateICmpNE(
      B.CreateAnd(Mask, ConstantInt::get(Mask->getType(), FeatureINITIALIZED)),
      ConstantInt::get(Mask->getType(), 0));
  B.CreateCondBr(Initialized, Select, Uninitialized);

  B.SetInsertPoint(Uninitialized);
  createTailReturn(B, *Resolver, BaselineImpl, CallInst::TCK_MustTail);

  B.SetInsertPoint(Select);
  Value *Selected = BaselineImpl;
  for (const Variant &V : Variants) {
    Value *Required = ConstantInt::get(Mask->getType(), V.RuntimeRequired);
    Value *Supported = B.CreateICmpEQ(B.CreateAnd(Mask, Required), Required);
    Selected = B.CreateSelect(Supported, V.F, Selected);
  }
  StoreInst *Publish = B.CreateStore(Selected, Slot);
  Publish->setAtomic(AtomicOrdering::Monotonic);
  Publish->setAlignment(Slot->getAlign().valueOrOne());
  createTailReturn(B, *Resolver, Selected, CallInst::TCK_MustTail);
  return Error::success();
}

} // namespace

Error llvm::goallc::finalizeCPUFeatureTailTransfers(Function &F) {
  if (!F.hasFnAttribute(TailTransferAttr))
    return Error::success();
  F.removeFnAttr(TailTransferAttr);
  for (BasicBlock &BB : F) {
    for (Instruction &I : BB) {
      auto *Call = dyn_cast<CallInst>(&I);
      if (!Call)
        continue;

      // Only a transfer which still returns the exact call result can use
      // musttail at codegen. This check also keeps the internal contract
      // fail-closed if the synthetic function shape changes later.
      auto *Ret = dyn_cast_or_null<ReturnInst>(Call->getNextNode());
      bool ReturnsCall =
          Ret && (Call->getType()->isVoidTy() ? Ret->getReturnValue() == nullptr
                                              : Ret->getReturnValue() == Call);
      if (Call->isTailCall() && ReturnsCall)
        Call->setTailCallKind(CallInst::TCK_MustTail);
    }
  }
  return Error::success();
}

Error llvm::goallc::runEarlyIRPipeline(Module &M) {
  if (M.getNamedMetadata(DoneMD))
    return Error::success();
  if (!M.getNamedMetadata(ConfigMD))
    return Error::success();

  Expected<CPUConfig> Config = getCPUConfig(M);
  if (!Config)
    return Config.takeError();

  SmallVector<Function *, 16> Candidates;
  for (Function &F : M)
    if (!F.isDeclaration() && F.hasFnAttribute(MultiversionAttr))
      Candidates.push_back(&F);

  if (!Candidates.empty()) {
    GlobalVariable *RuntimeMask = M.getNamedGlobal(RuntimeFeatureMask);
    if (!RuntimeMask)
      return createStringError(
          inconvertibleErrorCode(),
          "GoALLC CPU multiversioning requires runtime.goallcCPUFeatures");
    for (Function *F : Candidates)
      if (Error Err = multiversionFunction(*F, *Config, *RuntimeMask))
        return Err;
  }

  NamedMDNode *Done = M.getOrInsertNamedMetadata(DoneMD);
  Done->addOperand(MDNode::get(M.getContext(),
                               MDString::get(M.getContext(), "goallc.cpu.v1")));
  if (verifyModule(M, &errs()))
    return createStringError(inconvertibleErrorCode(),
                             "GoALLC CPU multiversioning produced invalid IR");
  return Error::success();
}

PreservedAnalyses llvm::goallc::CPUFeaturesPass::run(Module &M,
                                                     ModuleAnalysisManager &) {
  if (Error Err = runEarlyIRPipeline(M)) {
    M.getContext().emitError(toString(std::move(Err)));
    return PreservedAnalyses::none();
  }
  return PreservedAnalyses::none();
}
