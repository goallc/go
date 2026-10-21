// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

#include "GoALLCLTO.h"
#include "GoALLCPlugin.h"
#include "llvm/ADT/DenseSet.h"
#include "llvm/ADT/StringMap.h"
#include "llvm/BinaryFormat/GoObj.h"
#include "llvm/Bitcode/BitcodeWriter.h"
#include "llvm/IR/Constants.h"
#include "llvm/IR/DiagnosticInfo.h"
#include "llvm/IR/DiagnosticPrinter.h"
#include "llvm/IR/InstIterator.h"
#include "llvm/IR/LLVMContext.h"
#include "llvm/IR/Module.h"
#include "llvm/IR/Verifier.h"
#include "llvm/IRReader/IRReader.h"
#include "llvm/LTO/LTO.h"
#include "llvm/Passes/PassBuilder.h"
#include "llvm/Support/FileSystem.h"
#include "llvm/Support/JSON.h"
#include "llvm/Support/SourceMgr.h"
#include "llvm/Support/TargetSelect.h"
#include "llvm/Support/ToolOutputFile.h"
#include "llvm/Transforms/Utils/Cloning.h"
#include "llvm/Transforms/Utils/ModuleUtils.h"
#include <cstdlib>
#include <cstring>
#include <memory>
#include <mutex>
#include <vector>

using namespace llvm;
static constexpr StringLiteral OwnerMD = "goallc.lto.owner";
// Go 1.27 cmd/internal/goobj SymFlagLinknameStd (not in older LLVM headers).
static constexpr uint32_t GoSymFlagLinknameStd = 1u << 5;

enum ConfigField {
  Format,
  OS,
  Arch,
  Version,
  ArchKey,
  ArchValue,
  BuildID,
  PackagePrefix,
  IsMain,
  Shared,
  StandardLibrary,
  Experiments,
  Fingerprint,
  ConfigFields
};

static Error fail(const Twine &Message) {
  return createStringError(inconvertibleErrorCode(), Message);
}

struct Package {
  MDNode *Config;
  std::string Prefix;
  std::vector<Module::GlobalAsmFragment> ModuleAsm;
  StringMap<SmallVector<MDNode *>> Settings;
  SmallVector<std::pair<std::string, unsigned>> Linknames;
};

static std::optional<unsigned> owner(const GlobalObject &G) {
  if (const MDNode *N = G.getMetadata(OwnerMD))
    return mdconst::extract<ConstantInt>(N->getOperand(0))->getZExtValue();
  return std::nullopt;
}

static void givePackageName(GlobalObject &G, StringRef Prefix) {
  StringRef Name = G.getName();
  G.setMetadata(
      "goallc.lto.name",
      MDNode::get(G.getContext(), MDString::get(G.getContext(), Name)));
  bool ABI0 = Name.consume_back(GoObj::ABI0SymbolSuffix);
  StringRef Variant = GoObj::getSymbolSuffix(Name);
  if (Variant.starts_with(GoObj::FMVSymbolSuffixPrefix))
    Name = Name.drop_back(Variant.size());
  else
    Variant = {};
  G.setName(Name + ".goallc.lto." + Prefix + Variant +
            (ABI0 ? GoObj::ABI0SymbolSuffix : StringRef()));
}

static bool clonedConstant(const GlobalValue &G) {
  const auto *V = dyn_cast<GlobalVariable>(&G);
  return V && V->isConstant() && !V->getMetadata("goobj.symbol.index") &&
         ((V->hasLocalLinkage() && V->hasGlobalUnnamedAddr()) ||
          V->getMetadata("goobj.content_hash"));
}

// These identities are observable from assembly and unselected packages.
// LLVM is free to optimize bodies, but cannot replace or duplicate the Go
// linker symbols. In particular, a specialized clone must not inherit its
// source function's package symbol index.
struct SymbolIdentity {
  unsigned Owner;
  MDNode *Index;
};

static Error checkSymbolIdentities(Module &M,
                                   const StringMap<SymbolIdentity> &Symbols) {
  if (!M.alias_empty() || !M.ifunc_empty())
    return fail("optimization produced an alias or ifunc");
  DenseSet<std::pair<unsigned, uint64_t>> Indices;
  for (GlobalObject &G : M.global_objects()) {
    if (G.isDeclaration() || G.getName().starts_with("llvm."))
      continue;
    auto O = owner(G);
    if (!O && !clonedConstant(G))
      return fail(
          "optimization produced a definition without package ownership: " +
          G.getName());
    if (auto *Index = G.getMetadata("goobj.symbol.index")) {
      if (!O ||
          !Indices
               .insert({*O, mdconst::extract<ConstantInt>(Index->getOperand(0))
                                ->getZExtValue()})
               .second)
        return fail("duplicate package symbol index after optimization: " +
                    G.getName());
    }
  }
  for (const auto &S : Symbols) {
    auto *G = dyn_cast_or_null<GlobalObject>(M.getNamedValue(S.getKey()));
    if (!G || G->isDeclaration() || owner(*G) != S.second.Owner ||
        G->getMetadata("goobj.symbol.index") != S.second.Index)
      return fail("optimization changed retained Go symbol identity: " +
                  S.getKey());
  }
  return Error::success();
}

static bool referencesLinkname(const Value *V) {
  if (auto *G = dyn_cast<GlobalValue>(V)) {
    StringRef Name = G->getName();
    Name.consume_back(GoObj::ABI0SymbolSuffix);
    return GoObj::getSymbolSuffix(Name) == GoObj::LinknameSymbolSuffix;
  }
  if (auto *K = dyn_cast<Constant>(V))
    for (const Value *Op : K->operands())
      if (referencesLinkname(Op))
        return true;
  return false;
}

static Error prepareInput(Module &M) {
  LoopAnalysisManager LAM;
  FunctionAnalysisManager FAM;
  CGSCCAnalysisManager CGAM;
  ModuleAnalysisManager MAM;
  PassBuilder PB;
  registerGoALLCPassBuilderCallbacks(PB);
  PB.registerModuleAnalyses(MAM);
  PB.registerCGSCCAnalyses(CGAM);
  PB.registerFunctionAnalyses(FAM);
  PB.registerLoopAnalyses(LAM);
  PB.crossRegisterProxies(LAM, FAM, CGAM, MAM);
  ModulePassManager PM;
  if (Error E = PB.parsePassPipeline(PM, "goallc-cpu-features"))
    return fail(toString(std::move(E)));
  PM.run(M, MAM);
  if (verifyModule(M, &errs()))
    return fail("invalid optimized module");
  return Error::success();
}

static Error writeSingleModule(Module &M, ArrayRef<Package> Packages,
                               StringRef OutDir, bool SaveIR) {
  LLVMContext &C = M.getContext();
  unsigned Main = Packages.size();
  json::Array PackageRecords, ReferenceRecords;
  for (unsigned I = 0; I != Packages.size(); ++I) {
    const Package &P = Packages[I];
    if (cast<MDString>(P.Config->getOperand(IsMain))->getString() == "1") {
      if (Main != Packages.size())
        return fail("single-object FullLTO requires exactly one main package");
      Main = I;
    }
    json::Array Imports, Cgo;
    for (const auto &Fragment : P.ModuleAsm)
      Cgo.push_back(StringRef(Fragment.Asm)
                        .trim()
                        .drop_front(StringRef(".goobj.cgo ").size()));
    auto It = P.Settings.find("goobj.imports");
    if (It != P.Settings.end())
      for (MDNode *E : It->second)
        Imports.push_back(json::Object{
            {"path", cast<MDString>(E->getOperand(0))->getString()},
            {"prefix", cast<MDString>(E->getOperand(1))->getString()},
            {"fingerprint", cast<MDString>(E->getOperand(2))->getString()}});
    StringRef PackageName;
    auto DebugIt = P.Settings.find("goobj.debug.config");
    if (DebugIt != P.Settings.end() && !DebugIt->second.empty() &&
        DebugIt->second.front()->getNumOperands() >= 4)
      PackageName =
          cast<MDString>(DebugIt->second.front()->getOperand(3))->getString();
    PackageRecords.push_back(json::Object{
        {"prefix", P.Prefix},
        {"name", PackageName},
        {"std",
         cast<MDString>(P.Config->getOperand(StandardLibrary))->getString() ==
             "1"},
        {"fingerprint",
         cast<MDString>(P.Config->getOperand(Fingerprint))->getString()},
        {"imports", std::move(Imports)},
        {"cgo", std::move(Cgo)}});
    for (const auto &[Name, ABI] : P.Linknames)
      ReferenceRecords.push_back(
          json::Object{{"package", I}, {"name", Name}, {"abi", ABI}});
  }
  if (Main == Packages.size())
    return fail("single-object FullLTO requires a main package");

  json::Array Symbols;
  uint32_t NextIndex = 0;
  for (GlobalObject &G : M.global_objects()) {
    if (G.isDeclaration() || G.getName().starts_with("llvm."))
      continue;
    auto O = owner(G);
    if (!O)
      continue; // Optimizer-created content-addressed constants have no owner.
    unsigned ABI = GoObj::SymABI0;
    if (const auto *F = dyn_cast<Function>(&G))
      ABI = F->getCallingConv() == CallingConv::GoABI0 ? GoObj::SymABI0
                                                       : GoObj::SymABIInternal;
    MDNode *Index = G.getMetadata("goobj.symbol.index");
    if (Index && Index->getNumOperands() == 2)
      ABI = mdconst::extract<ConstantInt>(Index->getOperand(1))->getZExtValue();
    StringRef Name = G.getName();
    Name.consume_back(GoObj::ABI0SymbolSuffix);
    StringRef Suffix = GoObj::getSymbolSuffix(Name);
    if (Suffix.starts_with(GoObj::FMVSymbolSuffixPrefix)) {
      ABI = GoObj::SymABIstatic;
      Name = Name.drop_back(Suffix.size());
    }
    json::Object S{{"package", *O}, {"name", Name}, {"abi", ABI}};
    if (auto *Original = G.getMetadata("goallc.lto.name")) {
      StringRef OriginalName =
          cast<MDString>(Original->getOperand(0))->getString();
      OriginalName.consume_back(GoObj::ABI0SymbolSuffix);
      StringRef Variant = GoObj::getSymbolSuffix(OriginalName);
      if (Variant.starts_with(GoObj::FMVSymbolSuffixPrefix))
        OriginalName = OriginalName.drop_back(Variant.size());
      S["original_name"] = OriginalName;
    }
    if (Index) {
      S["original_index"] =
          mdconst::extract<ConstantInt>(Index->getOperand(0))->getZExtValue();
      S["index"] = NextIndex;
      Metadata *Fields[] = {
          ConstantAsMetadata::get(
              ConstantInt::get(Type::getInt32Ty(C), NextIndex++)),
          ConstantAsMetadata::get(ConstantInt::get(Type::getInt16Ty(C), ABI))};
      G.setMetadata("goobj.symbol.index", MDNode::get(C, Fields));
    }
    Symbols.push_back(std::move(S));
    G.setMetadata("goallc.lto.name", nullptr);
  }

  const Package &P = Packages[Main];
  for (StringRef Name :
       {"goobj.config", "goobj.static_rodata_type", "goallc.cpu.config"}) {
    auto It = P.Settings.find(Name);
    if (It != P.Settings.end())
      for (MDNode *E : It->second)
        M.getOrInsertNamedMetadata(Name)->addOperand(E);
  }
  // Library loading and the original import DAG are supplied by the manifest.
  // Putting the union of imports on main would introduce spurious cycles.
  M.getOrInsertNamedMetadata("goobj.imports");
  MDNode *DebugConfig = nullptr;
  auto MainDebug = P.Settings.find("goobj.debug.config");
  if (MainDebug != P.Settings.end() && !MainDebug->second.empty())
    DebugConfig = MainDebug->second.front();
  for (const Package &Pkg : Packages) {
    auto It = Pkg.Settings.find("goobj.debug.config");
    if (It != Pkg.Settings.end() && !It->second.empty()) {
      MDNode *N = It->second.front();
      if (!DebugConfig || N->getNumOperands() > DebugConfig->getNumOperands())
        DebugConfig = N;
    }
  }
  if (DebugConfig)
    M.getOrInsertNamedMetadata("goobj.debug.config")->addOperand(DebugConfig);
  if (auto *N = M.getNamedMetadata("goobj.gotype")) {
    SmallVector<MDNode *> Records;
    for (MDNode *E : N->operands()) {
      auto *V = dyn_cast_or_null<ValueAsMetadata>(E->getOperand(0));
      auto *G = V ? dyn_cast<GlobalObject>(V->getValue()) : nullptr;
      unsigned RecordOwner =
          mdconst::extract<ConstantInt>(E->getOperand(2))->getZExtValue();
      if (!G || G->isDeclaration() || owner(*G) != RecordOwner)
        continue;
      Metadata *Fields[] = {E->getOperand(0), E->getOperand(1)};
      Records.push_back(MDNode::get(C, Fields));
    }
    N->clearOperands();
    for (MDNode *E : Records)
      N->addOperand(E);
  }
  for (GlobalObject &G : M.global_objects())
    G.setMetadata(OwnerMD, nullptr);
  for (StructType *T : M.getIdentifiedStructTypes())
    if (T->hasName() && (T->getName() == "go.abi.pad" ||
                         T->getName().starts_with("go.abi.pad."))) {
      if (auto *Old = StructType::getTypeByName(C, "go.abi.pad");
          Old && Old != T)
        Old->setName("go.abi.pad.saved");
      T->setName("go.abi.pad");
    }
  if (verifyModule(M, &errs()))
    return fail("invalid single-object FullLTO module");
  if (std::error_code EC = sys::fs::create_directories(OutDir))
    return fail(EC.message());
  std::error_code EC;
  ToolOutputFile BC((OutDir + "/program.bc").str(), EC, sys::fs::OF_None);
  if (EC)
    return fail(EC.message());
  WriteBitcodeToFile(M, BC.os());
  BC.keep();
  ToolOutputFile Manifest((OutDir + "/program.json").str(), EC,
                          sys::fs::OF_Text);
  if (EC)
    return fail(EC.message());
  Manifest.os() << json::Value(
      json::Object{{"version", 1},
                   {"main", Main},
                   {"packages", std::move(PackageRecords)},
                   {"symbols", std::move(Symbols)},
                   {"linknames", std::move(ReferenceRecords)}});
  Manifest.keep();
  if (SaveIR) {
    ToolOutputFile IR((OutDir + "/program.ll").str(), EC, sys::fs::OF_Text);
    if (EC)
      return fail(EC.message());
    M.print(IR.os(), nullptr);
    IR.keep();
  }
  return Error::success();
}

static Error resolveGoBSS(ArrayRef<std::unique_ptr<Module>> Modules) {
  // Normalize Go named BSS before LLVM resolves and combines the inputs.
  // These placeholders follow Go linker rules rather than LLVM common linkage.
  StringMap<GlobalObject *> Definitions;
  auto LinknameBSS = [](GlobalVariable &G) {
    auto *Flags = G.getMetadata("goobj.symbol.flags");
    return G.hasInitializer() && G.getInitializer()->isNullValue() && Flags &&
           (mdconst::extract<ConstantInt>(Flags->getOperand(1))
                ->getZExtValue() &
            (GoObj::SymFlagLinkname | GoSymFlagLinknameStd));
  };
  auto BSS = [](GlobalVariable &G) {
    return G.hasInitializer() && G.getInitializer()->isNullValue() &&
           G.getMetadata("goobj.symbol.nonpackage") &&
           (G.getSection() == ".bss" || G.getSection() == ".noptrbss");
  };
  for (auto &M : Modules)
    for (GlobalObject &G : M->global_objects()) {
      if (G.isDeclaration() || G.hasLocalLinkage())
        continue;
      auto [It, Inserted] = Definitions.try_emplace(G.getName(), &G);
      if (Inserted)
        continue;
      GlobalObject *Old = It->second;
      auto *NewVar = dyn_cast<GlobalVariable>(&G);
      auto *OldVar = dyn_cast<GlobalVariable>(Old);
      if (NewVar && LinknameBSS(*NewVar) && isa<Function>(Old)) {
        NewVar->setInitializer(nullptr);
        continue;
      }
      if (OldVar && LinknameBSS(*OldVar) && isa<Function>(G)) {
        OldVar->setInitializer(nullptr);
        It->second = &G;
        continue;
      }
      if (!NewVar || !OldVar || (!BSS(*NewVar) && !BSS(*OldVar)) ||
          G.getMetadata("goobj.symbol.index") ||
          Old->getMetadata("goobj.symbol.index"))
        continue;
      const DataLayout &DL = M->getDataLayout();
      bool NewWins =
          BSS(*OldVar) &&
          (!BSS(*NewVar) || DL.getTypeAllocSize(NewVar->getValueType()) >
                                DL.getTypeAllocSize(OldVar->getValueType()));
      GlobalVariable &Winner = NewWins ? *NewVar : *OldVar;
      GlobalVariable &Loser = NewWins ? *OldVar : *NewVar;
      if (DL.getTypeAllocSize(Winner.getValueType()) <
          DL.getTypeAllocSize(Loser.getValueType()))
        return fail("BSS declaration exceeds initialized definition: " +
                    G.getName());
      Loser.setInitializer(nullptr);
      Loser.setLinkage(GlobalValue::ExternalLinkage);
      SmallVector<std::pair<unsigned, MDNode *>> MDs;
      Loser.getAllMetadata(MDs);
      for (auto [Kind, N] : MDs)
        Loser.setMetadata(Kind, nullptr);
      It->second = &Winner;
    }

  return Error::success();
}

static Error runFullLTO(ArrayRef<std::unique_ptr<Module>> Modules,
                        MutableArrayRef<Package> Packages, StringRef Pipeline,
                        StringRef OutDir, bool SaveIR) {
  LLVMContext &C = Modules.front()->getContext();
  // LTO reads bitcode into its own context. Transport package-scoped metadata
  // with each input instead of keeping pointers into the input LLVMContext.
  std::vector<SmallVector<char, 0>> Buffers(Modules.size());
  std::vector<std::unique_ptr<lto::InputFile>> Files;
  for (unsigned I = 0; I != Modules.size(); ++I) {
    Module &M = *Modules[I];
    SmallVector<Metadata *> Settings;
    for (auto &Entry : Packages[I].Settings) {
      SmallVector<Metadata *> Fields{MDString::get(C, Entry.getKey())};
      llvm::append_range(Fields, Entry.second);
      Settings.push_back(MDNode::get(C, Fields));
    }
    Metadata *Record[] = {
        ConstantAsMetadata::get(ConstantInt::get(Type::getInt32Ty(C), I)),
        Packages[I].Config, MDNode::get(C, Settings)};
    M.getOrInsertNamedMetadata("goallc.lto.packages")
        ->addOperand(MDNode::get(C, Record));
    raw_svector_ostream OS(Buffers[I]);
    WriteBitcodeToFile(M, OS);
    auto File = lto::InputFile::create(
        MemoryBufferRef(StringRef(Buffers[I].data(), Buffers[I].size()),
                        M.getModuleIdentifier()));
    if (!File)
      return File.takeError();
    Files.push_back(std::move(*File));
  }

  // Describe the link to LLVM. Keep externally named Go definitions visible:
  // ordinary assembly and packages outside this LTO unit can reference them.
  // Local indexed identities are retained separately by llvm.compiler.used.
  struct Definition {
    unsigned File;
    const lto::InputFile::Symbol *Sym;
  };
  StringMap<Definition> Prevailing;
  for (unsigned I = 0; I != Files.size(); ++I)
    for (const auto &S : Files[I]->symbols()) {
      if (S.isUndefined())
        continue;
      auto [It, Inserted] =
          Prevailing.try_emplace(S.getName(), Definition{I, &S});
      if (Inserted)
        continue;
      const auto &Old = *It->second.Sym;
      auto Rank = [](const lto::InputFile::Symbol &S) {
        return S.isCommon() ? 1 : S.isWeak() ? 0 : 2;
      };
      if (Rank(S) == 2 && Rank(Old) == 2)
        return fail("duplicate symbol: " + S.getName());
      if (Rank(S) > Rank(Old) || (S.isCommon() && Old.isCommon() &&
                                  S.getCommonSize() > Old.getCommonSize()))
        It->second = {I, &S};
    }

  static std::once_flag TargetsInitialized;
  std::call_once(TargetsInitialized, [] {
    InitializeAllTargetInfos();
    InitializeAllTargets();
    InitializeAllTargetMCs();
  });
  // The public entry uses lto<O2>; validate the test harness override before
  // LTOBackend's fatal-error path for malformed custom pipelines.
  ModulePassManager ValidationPipeline;
  PassBuilder PB;
  if (Error E = PB.parsePassPipeline(ValidationPipeline, Pipeline))
    return E;
  lto::Config Conf;
  Conf.OptPipeline = Pipeline.str();
  Conf.ShouldDiscardValueNames = false;
  std::string Diagnostics;
  bool DiagnosticError = false;
  Conf.DiagHandler = [&](const DiagnosticInfo &DI) {
    DiagnosticError |= DI.getSeverity() == DS_Error;
    raw_string_ostream OS(Diagnostics);
    DiagnosticPrinterRawOStream Printer(OS);
    DI.print(Printer);
    OS << '\n';
  };
  StringMap<SymbolIdentity> Symbols;
  Conf.PreOptModuleHook = [&](unsigned, const Module &M) {
    for (const GlobalObject &G : M.global_objects())
      if (!G.isDeclaration() && !G.getName().starts_with("llvm."))
        if (auto *Index = G.getMetadata("goobj.symbol.index"))
          Symbols.try_emplace(G.getName(), SymbolIdentity{*owner(G), Index});
    return true;
  };
  std::string OutputError;
  bool WroteOutput = false;
  Conf.PostOptModuleHook = [&](unsigned, const Module &M) {
    // Hooks expose a const module. Work on a clone when restoring the GoObj
    // output contract, and stop before LLVM's native object code generation.
    auto Combined = CloneModule(M);
    auto *Records = Combined->getNamedMetadata("goallc.lto.packages");
    for (MDNode *Record : Records->operands()) {
      unsigned I =
          mdconst::extract<ConstantInt>(Record->getOperand(0))->getZExtValue();
      Package &P = Packages[I];
      P.Config = cast<MDNode>(Record->getOperand(1));
      P.Settings.clear();
      for (const MDOperand &Setting :
           cast<MDNode>(Record->getOperand(2))->operands()) {
        auto *Fields = cast<MDNode>(Setting);
        auto &Entries =
            P.Settings[cast<MDString>(Fields->getOperand(0))->getString()];
        for (unsigned J = 1; J != Fields->getNumOperands(); ++J)
          Entries.push_back(cast<MDNode>(Fields->getOperand(J)));
      }
    }
    Combined->eraseNamedMetadata(Records);
    if (Error E = checkSymbolIdentities(*Combined, Symbols)) {
      OutputError = toString(std::move(E));
      return false;
    }
    for (GlobalObject &G : Combined->global_objects()) {
      if (!G.isDeclaration())
        G.setMetadata("goobj.import", nullptr);
      if (G.hasLocalLinkage() && G.getMetadata("goobj.symbol.index"))
        G.setLinkage(GlobalValue::ExternalLinkage);
    }
    if (Error E = writeSingleModule(*Combined, Packages, OutDir, SaveIR))
      OutputError = toString(std::move(E));
    WroteOutput = true;
    return false;
  };
  lto::LTO LTO(std::move(Conf));
  for (unsigned I = 0; I != Files.size(); ++I) {
    SmallVector<lto::SymbolResolution> Resolutions;
    for (const auto &S : Files[I]->symbols()) {
      lto::SymbolResolution R;
      R.Prevailing =
          !S.isUndefined() && Prevailing.lookup(S.getName()).File == I;
      R.VisibleToRegularObj = true;
      R.FinalDefinitionInLinkageUnit = Prevailing.count(S.getName()) != 0;
      Resolutions.push_back(R);
    }
    if (Error E = LTO.add(std::move(Files[I]), Resolutions))
      return E;
  }
  if (Error E = LTO.run([](unsigned, const Twine &)
                            -> Expected<std::unique_ptr<CachedFileStream>> {
        return fail("unexpected FullLTO native code generation");
      }))
    return E;
  if (!OutputError.empty())
    return fail(OutputError);
  if (DiagnosticError)
    return fail(Diagnostics);
  if (!WroteOutput)
    return fail("FullLTO did not produce an optimized module: " + Diagnostics);
  return Error::success();
}

Error goallc::runLTO(ArrayRef<std::string> Inputs, StringRef OutDir,
                     StringRef Pipeline, bool SaveIR, bool Thin) {
  if (Thin)
    return fail("ThinLTO is not available in this backend yet");
  if (Inputs.empty())
    return fail("LTO requires at least one input");
  LLVMContext C;
  std::vector<Package> Packages;
  std::vector<std::unique_ptr<Module>> Modules;
  for (unsigned I = 0; I != Inputs.size(); ++I) {
    SMDiagnostic Diag;
    auto M = parseIRFile(Inputs[I], Diag, C);
    if (!M) {
      std::string Message;
      raw_string_ostream OS(Message);
      Diag.print("LTO", OS);
      return fail(Message);
    }
    if (!Modules.empty() &&
        (Modules.front()->getTargetTriple() != M->getTargetTriple() ||
         Modules.front()->getDataLayout() != M->getDataLayout()))
      return fail("incompatible target or data layout");
    if (Error E = prepareInput(*M))
      return E;
    // A linkname can be legal only from its originating package (including
    // runtime's private allowlist). Keep such bodies in that package instead
    // of disabling the Go linker's checks after cross-package inlining.
    for (Function &F : *M)
      for (Instruction &Inst : instructions(F))
        for (const Value *Op : Inst.operands())
          if (referencesLinkname(Op)) {
            F.removeFnAttr(Attribute::AlwaysInline);
            F.addFnAttr(Attribute::NoInline);
          }
    auto *Config = M->getNamedMetadata("goobj.config");
    if (!Config || Config->getNumOperands() != 1 ||
        Config->getOperand(0)->getNumOperands() != ConfigFields)
      return fail("input requires one compiler GoObj configuration");
    Package P;
    for (GlobalObject &G : M->global_objects())
      if (G.isDeclaration() && referencesLinkname(&G)) {
        StringRef Name = G.getName();
        bool ABI0 = Name.consume_back(GoObj::ABI0SymbolSuffix);
        Name.consume_back(GoObj::LinknameSymbolSuffix);
        unsigned ABI =
            isa<Function>(G) && !ABI0 ? GoObj::SymABIInternal : GoObj::SymABI0;
        P.Linknames.emplace_back(Name.str(), ABI);
      }
    for (GlobalVariable &G : M->globals())
      if (G.hasInitializer() && G.getInitializer()->isNullValue())
        if (auto *Flags = G.getMetadata("goobj.symbol.flags")) {
          uint64_t Flag2 = mdconst::extract<ConstantInt>(Flags->getOperand(1))
                               ->getZExtValue();
          if (Flag2 & (GoObj::SymFlagLinkname | GoSymFlagLinknameStd))
            P.Linknames.emplace_back(G.getName().str(), GoObj::SymABI0);
        }
    P.Config = Config->getOperand(0);
    for (unsigned Field = 0; Field != ConfigFields; ++Field)
      if (Field == Experiments ? !isa<MDNode>(P.Config->getOperand(Field))
                               : !isa<MDString>(P.Config->getOperand(Field)))
        return fail("invalid GoObj configuration field");
    if (cast<MDString>(P.Config->getOperand(Format))->getString() !=
        "goallc.goobj")
      return fail("unsupported GoObj configuration format");
    P.Prefix = cast<MDString>(P.Config->getOperand(PackagePrefix))->getString();
    if (P.Prefix.empty())
      return fail("empty package prefix");
    // The GoObj producer classifies synthesized read-only data per module.
    // Importing FIPS bodies into another package (ThinLTO) or merging them
    // with non-FIPS code (FullLTO) loses that boundary. The driver compiles
    // these packages normally; reject explicit or stale bitcode inputs too.
    if (P.Prefix == "crypto/internal/fips140" ||
        StringRef(P.Prefix).starts_with("crypto/internal/fips140/") ||
        M->getNamedMetadata("goobj.static_rodata_type"))
      return fail("FIPS packages must use ordinary object compilation: " +
                  P.Prefix);
    for (const Package &Previous : Packages)
      if (P.Prefix == Previous.Prefix)
        return fail("duplicate package prefix: " + P.Prefix);
    if (M->getNamedMetadata("goobj.dynlink"))
      return fail("dynamic Go linking is not supported");
    if (!Packages.empty())
      for (unsigned Field :
           {Format, OS, Arch, Version, ArchKey, ArchValue, Shared, Experiments})
        if (P.Config->getOperand(Field) !=
            Packages[0].Config->getOperand(Field))
          return fail("incompatible GoObj configuration in " + P.Prefix);
    llvm::append_range(P.ModuleAsm, M->getModuleInlineAsm());
    for (const auto &Fragment : P.ModuleAsm) {
      StringRef Asm = StringRef(Fragment.Asm).trim();
      if (!Asm.empty() &&
          (!Asm.starts_with(".goobj.cgo ") || Asm.contains('\n')))
        return fail("unsupported module assembly in " + P.Prefix);
    }
    M->removeModuleInlineAsm();
    // These records describe the output package, not the combined program.
    for (StringRef Name :
         {"goobj.config", "goobj.imports", "goobj.debug.config",
          "goobj.static_rodata_type", "goallc.cpu.config"}) {
      if (auto *N = M->getNamedMetadata(Name)) {
        for (MDNode *E : N->operands())
          P.Settings[Name].push_back(E);
        M->eraseNamedMetadata(N);
      }
    }
    if (!M->alias_empty() || !M->ifunc_empty())
      return fail("aliases and ifuncs are not supported in " + P.Prefix);
    uint64_t NextIndex = 0;
    DenseSet<uint64_t> Indices;
    for (GlobalObject &G : M->global_objects())
      if (MDNode *Index = G.getMetadata("goobj.symbol.index")) {
        auto *Value =
            Index->getNumOperands() > 0
                ? mdconst::dyn_extract<ConstantInt>(Index->getOperand(0))
                : nullptr;
        if (!Value || Value->getBitWidth() != 32 ||
            (Index->getNumOperands() != 1 && Index->getNumOperands() != 2))
          return fail("invalid package symbol index: " + G.getName());
        if (Index->getNumOperands() == 2) {
          auto *ABI = mdconst::dyn_extract<ConstantInt>(Index->getOperand(1));
          if (!ABI || ABI->getBitWidth() != 16)
            return fail("invalid package symbol ABI: " + G.getName());
        }
        if (!G.isDeclaration() && !Indices.insert(Value->getZExtValue()).second)
          return fail("duplicate package symbol index: " + G.getName());
        NextIndex = std::max(NextIndex, 1 + Value->getZExtValue());
      }
    for (GlobalObject &G : M->global_objects()) {
      if (G.isDeclaration() || G.getName().starts_with("llvm."))
        continue;
      G.setMetadata(OwnerMD,
                    MDNode::get(C, ConstantAsMetadata::get(ConstantInt::get(
                                       Type::getInt32Ty(C), I))));
      // Content-addressed functions normally remain in their compiler
      // package. LTO can expose references to them in other packages before
      // their machine-code hashes exist. Give each original copy an indexed
      // identity; this preserves distinct bodies and package-local data.
      bool HashedFunction =
          isa<Function>(G) && G.getMetadata("goobj.content_addressable");
      if (HashedFunction) {
        givePackageName(G, P.Prefix);
        G.setMetadata("goobj.content_addressable", nullptr);
      }
      if ((G.hasLocalLinkage() || HashedFunction) && !clonedConstant(G) &&
          !G.getMetadata("goobj.symbol.index")) {
        if (NextIndex >= UINT32_MAX)
          return fail("too many package symbols");
        Metadata *Index[] = {
            ConstantAsMetadata::get(
                ConstantInt::get(Type::getInt32Ty(C), NextIndex++)),
            ConstantAsMetadata::get(ConstantInt::get(
                Type::getInt16Ty(C),
                HashedFunction
                    ? (cast<Function>(G).getCallingConv() == CallingConv::GoABI0
                           ? GoObj::SymABI0
                           : GoObj::SymABIInternal)
                    : GoObj::SymABIstatic))};
        G.setMetadata("goobj.symbol.index", MDNode::get(C, Index));
        G.setMetadata("goobj.symbol.nonpackage", nullptr);
      }
    }
    if (auto *N = M->getNamedMetadata("goobj.gotype")) {
      SmallVector<MDNode *> Records;
      for (MDNode *E : N->operands()) {
        if (E->getNumOperands() != 2)
          return fail("invalid gotype record");
        Metadata *Fields[] = {
            E->getOperand(0), E->getOperand(1),
            ConstantAsMetadata::get(ConstantInt::get(Type::getInt32Ty(C), I))};
        Records.push_back(MDNode::get(C, Fields));
      }
      N->clearOperands();
      for (MDNode *E : Records)
        N->addOperand(E);
    }
    // The IR linker can discard unreferenced local definitions before the
    // optimization pipeline runs. Preserve them while their package is still
    // separate, including symbols that only ordinary assembly can reference.
    SmallVector<GlobalValue *> Retained;
    for (GlobalObject &G : M->global_objects())
      if (!G.isDeclaration() && !G.getName().starts_with("llvm.") &&
          !clonedConstant(G))
        Retained.push_back(&G);
    appendToCompilerUsed(*M, Retained);
    Packages.push_back(std::move(P));
    M->setModuleIdentifier(Inputs[I]);
    Modules.push_back(std::move(M));
  }
  if (Error E = resolveGoBSS(Modules))
    return E;
  return runFullLTO(Modules, Packages, Pipeline, OutDir, SaveIR);
}

// The caller owns the returned error string and frees it with free().
extern "C" char *GoALLCLTO(const char *const *Inputs, size_t Count,
                           const char *OutDir, int SaveIR, int Thin) {
  std::vector<std::string> Paths;
  for (size_t I = 0; I != Count; ++I)
    Paths.emplace_back(Inputs[I]);
  if (Error E = goallc::runLTO(Paths, OutDir, Thin ? "thinlto<O2>" : "lto<O2>",
                               SaveIR, Thin))
    return strdup(toString(std::move(E)).c_str());
  return nullptr;
}
