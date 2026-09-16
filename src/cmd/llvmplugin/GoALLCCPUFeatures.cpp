// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCCPUFeatures.h"

#include "llvm/ADT/ArrayRef.h"
#include "llvm/ADT/STLExtras.h"
#include "llvm/ADT/SmallVector.h"
#include "llvm/ADT/StringMap.h"
#include "llvm/ADT/StringRef.h"
#include "llvm/BinaryFormat/GoObj.h"
#include "llvm/IR/Attributes.h"
#include "llvm/IR/Constants.h"
#include "llvm/IR/DebugInfoMetadata.h"
#include "llvm/IR/DerivedTypes.h"
#include "llvm/IR/Function.h"
#include "llvm/IR/GlobalVariable.h"
#include "llvm/IR/IRBuilder.h"
#include "llvm/IR/Instructions.h"
#include "llvm/IR/InstIterator.h"
#include "llvm/IR/IntrinsicInst.h"
#include "llvm/IR/Metadata.h"
#include "llvm/IR/Module.h"
#include "llvm/IR/Verifier.h"
#include "llvm/MC/MCSubtargetInfo.h"
#include "llvm/MC/TargetRegistry.h"
#include "llvm/Support/Alignment.h"
#include "llvm/Support/Error.h"
#include "llvm/Transforms/Utils/Cloning.h"
#include "llvm/Transforms/Utils/FunctionComparator.h"
#include "llvm/Transforms/Utils/Local.h"
#include "llvm/Transforms/Utils/ModuleUtils.h"
#include "llvm/Transforms/Utils/ValueMapper.h"

#include <cstdint>
#include <memory>
#include <string>
#include <utility>

using namespace llvm;

namespace {

constexpr StringLiteral ConfigMD = "goallc.cpu.config";
constexpr StringLiteral DoneMD = "goallc.cpu.fmv.done";
constexpr StringLiteral GuardMD = "goallc.cpu.guard";
constexpr StringLiteral RequiresMD = "goallc.cpu.requires";
constexpr StringLiteral RequireAnchorMD = "goallc.cpu.require-anchor";
constexpr StringLiteral AutoMD = "goallc.cpu.auto";
constexpr StringLiteral MultiversionAttr = "goallc.cpu.multiversion";
constexpr StringLiteral RuntimeFeatureMask = "runtime.goallcCPUFeatures";
constexpr StringLiteral GoResultsTupleAttr = "go_results_tuple";
constexpr StringLiteral GoObjDebugFuncsMD = "goobj.debug.funcs";
constexpr StringLiteral GoObjNonPackageMD = "goobj.symbol.nonpackage";
constexpr StringLiteral GoObjSymbolFlagsMD = "goobj.symbol.flags";
constexpr StringLiteral GoObjFuncInfoMD = "goobj.func.info";
constexpr StringLiteral GoNoSplitAttr = "go-nosplit";
constexpr StringLiteral TailTransferAttr = "goallc.cpu.tail-transfers";
constexpr StringLiteral DispatcherInlineMD = "goallc.cpu.dispatcher.inline";
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
  uint64_t Predicate;
};

struct CPUConfig {
  StringRef Arch;
  uint64_t Baseline;
};

// These are generated from the same predicate/capability registry as the Go
// compiler and runtime snapshot. Target capabilities never imply Go booleans.
#define GOALLC_CPU_BASELINE(Name, Capabilities) \
  constexpr uint64_t Name##Baseline = Capabilities;
#include "GoALLCCPUFeatures.def"
#undef GOALLC_CPU_BASELINE

constexpr Profile Profiles[] = {
#define GOALLC_CPU_PROFILE(Name, Suffix, Target, Arch, Predicate) \
  {Name, Suffix, Target, Arch, Predicate},
#include "GoALLCCPUFeatures.def"
#undef GOALLC_CPU_PROFILE
};

const Profile *findProfile(StringRef Name) {
  for (const Profile &P : Profiles)
    if (P.Name == Name)
      return &P;
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
    if (*Level == "v3")
      return CPUConfig{*Arch, V3Baseline};
    if (*Level == "v4")
      return CPUConfig{*Arch, V4Baseline};
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

  SmallVector<StringRef, 16> Parts;
  StringRef(Features).split(Parts, ',', -1, false);
  SmallVector<StringRef, 16> Added;
  Feature.split(Added, ',', -1, false);
  // Move the entire requested bundle to the end, overriding earlier +/-
  // settings and letting LLVM re-enable any implied dependencies.
  std::string Result;
  for (StringRef Part : Parts) {
    if (llvm::any_of(Added, [&](StringRef A) {
          return A.drop_front() == Part.drop_front();
        }))
      continue;
    if (!Result.empty())
      Result += ',';
    Result += Part;
  }
  if (!Result.empty())
    Result += ',';
  Result += Feature;
  F.addFnAttr("target-features", Result);
}

void addTargetFeatures(Function &F, ArrayRef<const Profile *> Profiles) {
  for (const Profile *P : Profiles)
    addTargetFeature(F, P->TargetFeature);
}

bool containsWideVector(Type *Ty) {
  if (auto *VT = dyn_cast<FixedVectorType>(Ty))
    return VT->getPrimitiveSizeInBits().getFixedValue() > 128;
  if (auto *AT = dyn_cast<ArrayType>(Ty))
    return containsWideVector(AT->getElementType());
  if (auto *ST = dyn_cast<StructType>(Ty))
    return !ST->isOpaque() && llvm::any_of(ST->elements(), containsWideVector);
  return false;
}

bool hasWideVectorRegisterCarrier(const Function &F) {
  if (containsWideVector(F.getReturnType()))
    return true;
  return llvm::any_of(F.args(), [](const Argument &Arg) {
    return containsWideVector(Arg.getType());
  });
}

Expected<bool> specializeGuards(Function &F, uint64_t Predicates) {
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
    bool Enabled = (Predicates & P->Predicate) == P->Predicate;
    Load->replaceAllUsesWith(ConstantInt::get(Load->getType(), Enabled));
    Load->eraseFromParent();
  }

  // Only the guard-specialized constant path needs folding here. Keep this
  // local and analysis-free: the enclosing module pass runs before the normal
  // PassBuilder pipeline has established cross-analysis proxies.
  for (BasicBlock &BB : F) {
    SimplifyInstructionsInBlock(&BB);
    // llvm.expect is an identity with a branch prediction hint. Its constant
    // operand must reach the terminator before removing unsupported feature
    // paths, even when no optimizing pipeline follows specialization (O0).
    for (Instruction &I : make_early_inc_range(BB)) {
      auto *II = dyn_cast<IntrinsicInst>(&I);
      if (!II || (II->getIntrinsicID() != Intrinsic::expect &&
                  II->getIntrinsicID() != Intrinsic::expect_with_probability) ||
          !isa<ConstantInt>(II->getArgOperand(0)))
        continue;
      II->replaceAllUsesWith(II->getArgOperand(0));
      II->eraseFromParent();
    }
    SimplifyInstructionsInBlock(&BB);
  }
  for (BasicBlock &BB : F)
    ConstantFoldTerminator(&BB, true);
  removeUnreachableBlocks(F);
  for (BasicBlock &BB : F)
    SimplifyInstructionsInBlock(&BB);
  return !Guards.empty();
}

Error validateRequirementAnchor(const Instruction &I) {
  const MDNode *Marker = I.getMetadata(RequireAnchorMD);
  if (!Marker)
    return Error::success();
  if (Marker->getNumOperands() != 0)
    return createStringError(inconvertibleErrorCode(),
                             "!goallc.cpu.require-anchor must be empty");
  const auto *Call = dyn_cast<IntrinsicInst>(&I);
  if (!Call || Call->getIntrinsicID() != Intrinsic::sideeffect ||
      !Call->getType()->isVoidTy() || !Call->arg_empty() ||
      Call->hasOperandBundles())
    return createStringError(
        inconvertibleErrorCode(),
        "!goallc.cpu.require-anchor must mark a void llvm.sideeffect() call "
        "without arguments or operand bundles");
  if (!I.getMetadata(RequiresMD))
    return createStringError(
        inconvertibleErrorCode(),
        "!goallc.cpu.require-anchor must have !goallc.cpu.requires");
  return Error::success();
}

Error verifyRequirements(Function &F, const CPUConfig &Config) {
  if (llvm::none_of(instructions(F), [](const Instruction &I) {
        return I.getMetadata(RequiresMD) != nullptr;
      }))
    return Error::success();
  // LLVM also removes continuations after noreturn calls and repairs PHIs.
  // Discard their dead requirements before validating surviving operations.
  removeUnreachableBlocks(F);
  // LLVM owns CPU defaults, feature implication and ordered +/- overrides.
  // Query that effective target instead of maintaining another ISA model.
  std::string ErrorMessage;
  const Triple &TT = F.getParent()->getTargetTriple();
  const Target *T = TargetRegistry::lookupTarget(TT, ErrorMessage);
  if (!T)
    return createStringError(inconvertibleErrorCode(), ErrorMessage);
  StringRef CPU = F.getFnAttribute("target-cpu").getValueAsString();
  if (CPU.empty()) {
    if (Config.Arch == "amd64")
      CPU = Config.Baseline == V4Baseline ? "x86-64-v4"
          : Config.Baseline == V3Baseline ? "x86-64-v3"
          : Config.Baseline == V2Baseline ? "x86-64-v2" : "x86-64";
    else
      CPU = "generic";
  }
  std::string Features;
  if (Config.Arch == "arm64" && (Config.Baseline & FeatureARM64LSE))
    Features = "+lse,";
  Features += F.getFnAttribute("target-features").getValueAsString();
  std::unique_ptr<MCSubtargetInfo> STI(
      T->createMCSubtargetInfo(TT, CPU, Features));
  if (!STI)
    return createStringError(inconvertibleErrorCode(),
                             "cannot query GoALLC CPU target features");
  // Explicit hardware guards have already been specialized. Automatic checks
  // mark only the unsupported operation's path unreachable, relying on the
  // program's CPU precondition. Supported clones retain the original IR
  // and register ABI; there are no extracted helpers or memory carriers.
  SmallVector<Instruction *, 8> Unsupported;
  for (BasicBlock &BB : F) {
    for (Instruction &I : BB) {
      if (!I.getMetadata(AutoMD))
        continue;
      Expected<StringRef> Name = getInstructionProfile(I, RequiresMD);
      if (!Name)
        return Name.takeError();
      const Profile *P = findProfile(*Name);
      if (!P || P->Arch != Config.Arch)
        return createStringError(inconvertibleErrorCode(),
                                 "invalid automatic CPU profile " + *Name);
      if (!STI->checkFeatures(P->TargetFeature)) {
        Unsupported.push_back(&I);
        break;
      }
    }
  }
  if (!Unsupported.empty()) {
    for (Instruction *I : Unsupported)
      changeToUnreachable(I);
    removeUnreachableBlocks(F);
  }
  SmallVector<Instruction *, 8> Anchors;
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
      if (P->Arch != Config.Arch || !STI->checkFeatures(P->TargetFeature))
        return createStringError(inconvertibleErrorCode(),
                                 "GoALLC CPU requirement " + *Name +
                                     " survives in function " + F.getName() +
                                     " without the required target features");
      if (I.getMetadata(RequireAnchorMD))
        Anchors.push_back(&I);
    }
  }
  // Keep source-local requirements alive through guard specialization and
  // local simplification, even when their value folds to an argument or a
  // constant. Once every requirement has passed, remove only these dedicated
  // anchors so they cannot inhibit later optimization. An unmarked
  // llvm.sideeffect call can carry unrelated semantics and must remain.
  for (Instruction *Anchor : Anchors)
    Anchor->eraseFromParent();
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

CallInst *createTailReturn(IRBuilder<> &B, Function &Signature, Value *Callee,
                           CallInst::TailCallKind Kind = CallInst::TCK_Tail) {
  CallInst *Call = createTailCall(B, Signature, Callee, Kind);
  if (Signature.getReturnType()->isVoidTy())
    B.CreateRetVoid();
  else
    B.CreateRet(Call);
  return Call;
}

struct Variant {
  Function *F;
  uint64_t RuntimeRequired;
};

// The GoObj writer consumes this suffix into a source-name/SymABIstatic
// identity. Keep it immediately before <ABI0> when cloning an ABI0 definition.
std::string fmvImplementationName(StringRef SourceName, StringRef Tag) {
  StringRef ABI0Suffix = GoObj::ABI0SymbolSuffix;
  const bool IsABI0 = SourceName.consume_back(ABI0Suffix);
  std::string Name =
      (SourceName + GoObj::FMVSymbolSuffixPrefix + Tag + ">").str();
  if (IsABI0)
    Name += ABI0Suffix;
  return Name;
}

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
  if (llvm::none_of(instructions(F), [](const Instruction &I) {
        return I.getMetadata(AutoMD) != nullptr;
      }))
    return Result;

  // Automatic requests precede LLVM's noreturn/CFG cleanup. Rebuild them
  // from live anchors, keeping conjunctions intact. Explicit observations
  // still request their own versions, even when they share an automatic bit.
  removeUnreachableBlocks(F);
  Result.clear();
  for (Instruction &I : instructions(F)) {
    StringRef Kind = I.getMetadata(GuardMD) ? GuardMD : RequiresMD;
    if (!I.getMetadata(GuardMD) && !I.getMetadata(AutoMD))
      continue;
    Expected<StringRef> Name = getInstructionProfile(I, Kind);
    if (!Name)
      return Name.takeError();
    const Profile *P = findProfile(*Name);
    if (!P || P->Arch != Arch)
      return createStringError(inconvertibleErrorCode(),
                               "invalid GoALLC CPU profile " + *Name);
    if (!llvm::is_contained(Result, P))
      Result.push_back(P);
  }
  return Result;
}

Expected<Function *> cloneVariant(Function &Source, StringRef Suffix,
                                  uint64_t Predicates,
                                  ArrayRef<const Profile *> EnabledProfiles) {
  const bool DuplicateOK = isGoObjDuplicateOK(Source);
  ValueToValueMapTy VMap;
  Function *Clone = CloneFunction(&Source, VMap);
  Clone->setName(fmvImplementationName(Source.getName(), Suffix));
  Clone->setLinkage(GlobalValue::InternalLinkage);
  Clone->setDSOLocal(true);
  Clone->removeFnAttr(MultiversionAttr);
  // This clone is the source function's physical execution frame. Retain the
  // complete FuncInfo node so every FuncID and FuncFlag keeps its runtime stack
  // semantics (gopanic, Wrapper, TopFrame, SPWrite, and future additions).
  eraseGoObjSourceSymbolIdentity(*Clone);
  if (DuplicateOK)
    markGoObjDuplicateOK(*Clone);
  addTargetFeatures(*Clone, EnabledProfiles);
  Expected<bool> Specialized = specializeGuards(*Clone, Predicates);
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
  Resolver->setName(fmvImplementationName(Source.getName(), "resolve"));
  Resolver->setLinkage(GlobalValue::InternalLinkage);
  Resolver->setDSOLocal(true);
  Resolver->removeFnAttr(MultiversionAttr);
  eraseGoObjSourceSymbolIdentity(*Resolver);
  // The resolver is frameless and tail-only, but it still executes Go code and
  // may be sampled or asynchronously preempted before the transfer. Retain the
  // source FuncInfo so GoObj emits valid PCSP and the source FuncID/FuncFlag
  // semantics also cover the first invocation.
  if (DuplicateOK)
    markGoObjDuplicateOK(*Resolver);
  eraseFunctionBodyPreservingMetadata(*Resolver);
  makeFramelessResolver(*Resolver);
  return Resolver;
}

Error multiversionFunction(Function &F, const CPUConfig &Config,
                           GlobalVariable &RuntimeMask,
                           ArrayRef<const Profile *> Requested) {
  const bool DuplicateOK = isGoObjDuplicateOK(F);
  const bool WideVectorABI = hasWideVectorRegisterCarrier(F);
  const bool Automatic = llvm::any_of(instructions(F), [](const Instruction &I) {
    return I.getMetadata(AutoMD) != nullptr;
  });
  Expected<Function *> BaselineOrErr = cloneVariant(
      F, "baseline", Config.Baseline, {});
  if (!BaselineOrErr)
    return BaselineOrErr.takeError();
  Function *BaselineImpl = *BaselineOrErr;
  if (Error Err =
          verifyRequirements(*BaselineImpl, Config))
    return Err;
  if (Error Err = registerGoObjDebugFunction(*BaselineImpl))
    return Err;
  if (Automatic)
    for (BasicBlock &BB : *BaselineImpl)
      SimplifyInstructionsInBlock(&BB);
  GlobalNumberState GlobalNumbers;

  SmallVector<const Profile *, 4> OrderedProfiles;
  for (const Profile &P : Profiles) {
    if (llvm::is_contained(Requested, &P))
      OrderedProfiles.push_back(&P);
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
      Required |= P->Predicate;
    }
    // Overlapping conjunctions can produce the same predicate union.
    if (llvm::any_of(Variants, [&](const Variant &V) {
          return V.RuntimeRequired == (Required & ~Config.Baseline);
        }))
      continue;
    uint64_t Predicates = Config.Baseline | Required;
    Expected<Function *> CloneOrErr =
        cloneVariant(F, Suffix, Predicates, EnabledProfiles);
    if (!CloneOrErr)
      return CloneOrErr.takeError();
    Function *Clone = *CloneOrErr;
    if (Error Err = verifyRequirements(*Clone, Config))
      return Err;
    if (Automatic) {
      for (BasicBlock &BB : *Clone)
        SimplifyInstructionsInBlock(&BB);
      // Reuse the already-legal baseline only when LLVM finds the same body
      // and ABI. Ignore just the extra target features, not other attributes.
      Attribute Features = Clone->getFnAttribute("target-features");
      Clone->removeFnAttr("target-features");
      if (BaselineImpl->hasFnAttribute("target-features"))
        Clone->addFnAttr(BaselineImpl->getFnAttribute("target-features"));
      bool Same =
          FunctionComparator(Clone, BaselineImpl, &GlobalNumbers).compare() == 0;
      Clone->removeFnAttr("target-features");
      if (Features.isValid())
        Clone->addFnAttr(Features);
      if (Same) {
        Clone->dropAllReferences();
        Clone->eraseFromParent();
        Clone = BaselineImpl;
      }
    }
    if (Clone != BaselineImpl)
      if (Error Err = registerGoObjDebugFunction(*Clone))
        return Err;
    // Keep the predicate mapping even when its implementation is shared:
    // another partial profile may otherwise win selection on this CPU.
    Variants.push_back({Clone, Required & ~Config.Baseline});
  }
  // A matching superset must win over its subsets, independent of profile order.
  llvm::sort(Variants, [](const Variant &A, const Variant &B) {
    return A.RuntimeRequired < B.RuntimeRequired;
  });

  LLVMContext &C = F.getContext();
  Module &M = *F.getParent();
  Function *Resolver = cloneResolver(F);
  // Implementation clones inherit the source target features. A scalar
  // dispatcher needs only the compile baseline; wide register carriers must
  // preserve the source features across the tail-transfer ABI.
  if (!WideVectorABI) {
    Resolver->removeFnAttr("target-features");
    F.removeFnAttr("target-features");
    if (Config.Arch == "arm64" && (Config.Baseline & FeatureARM64LSE)) {
      addTargetFeature(*Resolver, "+lse");
      addTargetFeature(F, "+lse");
    }
  }
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

  eraseFunctionBodyPreservingMetadata(F);
  F.removeFnAttr(MultiversionAttr);
  // The public function remains the source function and retains its complete
  // FuncInfo. It has no new physical frame: when left out of line it tail
  // transfers to a variant, and when inlined its synthetic debug scope is
  // removed immediately before statepoint rewriting below.
  makeFramelessDispatcher(F);
  BasicBlock *Entry = BasicBlock::Create(C, "entry", &F);
  IRBuilder<> B(Entry);
  setSyntheticDebugLocation(B, F);
  LoadInst *Target = B.CreateLoad(PointerType::getUnqual(C), Slot, "target");
  Target->setAtomic(AtomicOrdering::Monotonic);
  Target->setAlignment(Slot->getAlign().valueOrOne());
  MDNode *DispatcherMarker = MDNode::get(C, ArrayRef<Metadata *>());
  Target->setMetadata(DispatcherInlineMD, DispatcherMarker);
  CallInst *DispatchCall = createTailReturn(B, F, Target);
  DispatchCall->setMetadata(DispatcherInlineMD, DispatcherMarker);

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
  // Keep only this edge opaque to the normal inliner: the NOSPLIT resolver
  // must stay frameless, while variants remain eligible for legal inlining
  // into other feature-compatible callers, including Midway-generated code.
  CallInst *BaselineCall =
      createTailReturn(B, *Resolver, BaselineImpl, CallInst::TCK_MustTail);
  BaselineCall->setIsNoInline();

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
  // If the public dispatcher was inlined, drop exactly its synthetic inline
  // scope while retaining the real caller location and any outer inline chain.
  // An out-of-line dispatcher keeps the source location because it still is
  // the public source function. Clear the internal marker in both cases.
  for (BasicBlock &BB : F) {
    for (Instruction &I : BB) {
      if (!I.getMetadata(DispatcherInlineMD))
        continue;
      if (const DILocation *Loc = I.getDebugLoc().get())
        if (const DILocation *CallerLoc = Loc->getInlinedAt())
          I.setDebugLoc(DebugLoc(CallerLoc));
      I.setMetadata(DispatcherInlineMD, nullptr);
    }
  }

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

  SmallVector<std::pair<Function *, SmallVector<const Profile *, 4>>, 16> Candidates;
  for (Function &F : M) {
    if (F.isDeclaration())
      continue;
    // Check the marker contract before folding can erase a malformed anchor.
    // Capability requirements themselves are checked on each surviving path
    // after specialization.
    for (BasicBlock &BB : F)
      for (Instruction &I : BB)
        if (Error Err = validateRequirementAnchor(I))
          return Err;
    if (F.hasFnAttribute(MultiversionAttr)) {
      auto Requested = requestedProfiles(F, Config->Arch);
      if (!Requested)
        return Requested.takeError();
      if (!Requested->empty()) {
        Candidates.emplace_back(&F, std::move(*Requested));
        continue;
      }
      F.removeFnAttr(MultiversionAttr);
    }
    if (Error Err = verifyRequirements(F, *Config)) {
      return Err;
    }
  }

  if (!Candidates.empty()) {
    GlobalVariable *RuntimeMask = M.getNamedGlobal(RuntimeFeatureMask);
    if (!RuntimeMask)
      return createStringError(
          inconvertibleErrorCode(),
          "GoALLC CPU multiversioning requires runtime.goallcCPUFeatures");
    for (auto &[F, Requested] : Candidates)
      if (Error Err =
              multiversionFunction(*F, *Config, *RuntimeMask, Requested))
        return Err;
  }

  // Named debug records do not keep symbols alive through GlobalDCE. Publish
  // all roots once: rebuilding llvm.compiler.used for every generated version
  // retains quadratically many uniqued constants in large SIMD packages.
  if (NamedMDNode *Funcs = M.getNamedMetadata(GoObjDebugFuncsMD)) {
    SmallVector<GlobalValue *, 32> Roots;
    for (MDNode *Node : Funcs->operands())
      if (Node->getNumOperands() == 2)
        if (auto *F = mdconst::dyn_extract<Function>(Node->getOperand(1)))
          Roots.push_back(F);
    appendToCompilerUsed(M, Roots);
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

PreservedAnalyses
llvm::goallc::CPUFeaturesFinalizePass::run(Module &M,
                                           ModuleAnalysisManager &) {
  for (Function &F : M) {
    if (Error Err = finalizeCPUFeatureTailTransfers(F)) {
      M.getContext().emitError(toString(std::move(Err)));
      return PreservedAnalyses::none();
    }
  }
  return PreservedAnalyses::none();
}
