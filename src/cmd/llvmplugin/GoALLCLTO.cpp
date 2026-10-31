// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

#include "GoALLCLTO.h"
#include "llvm/ADT/DenseSet.h"
#include "llvm/ADT/StringMap.h"
#include "llvm/BinaryFormat/GoObj.h"
#include "llvm/Bitcode/BitcodeWriter.h"
#include "llvm/IR/Constants.h"
#include "llvm/IR/DiagnosticInfo.h"
#include "llvm/IR/DiagnosticPrinter.h"
#include "llvm/IR/LLVMContext.h"
#include "llvm/IR/Module.h"
#include "llvm/IR/Verifier.h"
#include "llvm/IRReader/IRReader.h"
#include "llvm/LTO/LTOBackend.h"
#include "llvm/Linker/Linker.h"
#include "llvm/Passes/PassBuilder.h"
#include "llvm/Support/Base64.h"
#include "llvm/Support/Endian.h"
#include "llvm/Support/FileSystem.h"
#include "llvm/Support/JSON.h"
#include "llvm/Support/SourceMgr.h"
#include "llvm/Support/TargetSelect.h"
#include "llvm/Support/ToolOutputFile.h"
#include "llvm/Transforms/Utils/ModuleUtils.h"
#include <cstdlib>
#include <cstring>
#include <memory>
#include <mutex>
#include <vector>

using namespace llvm;
// Go 1.27 cmd/internal/goobj SymFlagLinknameStd (not in older LLVM headers).
static constexpr uint32_t GoSymFlagLinknameStd = 1u << 5;

enum ConfigField {
  BuildID,
  PackagePrefix,
  IsMain,
  StandardLibrary,
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
  SmallVector<MDNode *> Imports;
  SmallVector<std::pair<std::string, unsigned>> Linknames;
};

// The native Go compiler already gives content-addressed function copies a
// #base64(8 bytes)# storage discriminator. GoObj removes it before computing
// the final text hash and writing names. Reuse that contract for distinct LTO
// copies: their storage identities must not become package-qualified symbols.
static std::string contentName(StringRef Name) {
  size_t Begin = Name.find('#');
  if (Begin != StringRef::npos && Name.size() >= Begin + 14 &&
      Name[Begin + 13] == '#')
    return (Name.take_front(Begin) + Name.drop_front(Begin + 14)).str();
  return Name.str();
}

static void giveContentName(GlobalObject &G, uint64_t &NextID) {
  std::string Canonical = contentName(G.getName());
  StringRef Name = Canonical;
  bool ABI0 = Name.consume_back(GoObj::ABI0SymbolSuffix);
  StringRef Variant = GoObj::getSymbolSuffix(Name);
  if (Variant.starts_with(GoObj::FMVSymbolSuffixPrefix))
    Name = Name.drop_back(Variant.size());
  else
    Variant = {};
  std::string Storage;
  do {
    char Bytes[8];
    support::endian::write64le(Bytes, NextID++);
    Storage = (Name + "#" + encodeBase64(StringRef(Bytes, 8)) + "#" + Variant +
               (ABI0 ? GoObj::ABI0SymbolSuffix : StringRef())).str();
  } while (G.getParent()->getNamedValue(Storage) && Storage != G.getName());
  G.setName(Storage);
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
  unsigned Package;
  MDNode *Index;
  std::string Section;
};

static Error checkSymbolIdentities(Module &M,
                                   const StringMap<SymbolIdentity> &Symbols) {
  if (!M.alias_empty() || !M.ifunc_empty())
    return fail("optimization produced an alias or ifunc");
  for (GlobalObject &G : M.global_objects()) {
    if (G.isDeclaration() || G.getName().starts_with("llvm."))
      continue;
    auto It = Symbols.find(G.getName());
    if (It == Symbols.end()) {
      if (!clonedConstant(G))
        return fail(
            "optimization produced a definition without package ownership: " +
            G.getName());
    } else if (G.getMetadata("goobj.symbol.index") != It->second.Index) {
      return fail("optimization changed retained Go symbol identity: " +
                  G.getName());
    }
  }
  for (const auto &S : Symbols)
    if (S.second.Index) {
      auto *G = dyn_cast_or_null<GlobalObject>(M.getNamedValue(S.getKey()));
      if (!G || G->isDeclaration())
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
  return false;
}

static Error writeSingleModule(Module &M, ArrayRef<Package> Packages,
                               StringRef OutDir, bool SaveIR,
                               const StringMap<SymbolIdentity> &Origins) {
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
    for (MDNode *E : P.Imports)
      Imports.push_back(json::Object{
          {"path", cast<MDString>(E->getOperand(0))->getString()},
          {"prefix", cast<MDString>(E->getOperand(1))->getString()},
          {"fingerprint", cast<MDString>(E->getOperand(2))->getString()}});
    PackageRecords.push_back(json::Object{
        {"prefix", P.Prefix},
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
    auto It = Origins.find(G.getName());
    if (It == Origins.end())
      continue; // Optimizer-created content-addressed constants have no owner.
    const SymbolIdentity &Origin = It->second;
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
    std::string Canonical;
    unsigned Owner = Origin.Package;
    if (isa<Function>(G) && G.getMetadata("goobj.content_addressable")) {
      Canonical = contentName(Name);
      Name = Canonical;
      // Content-addressed copies have no package-indexed external identity.
      // Attribute them to their source package when present, independently of
      // which importer supplied a copy. Anonymous compiler helpers use main.
      Owner = Main;
      size_t PrefixSize = 0;
      for (unsigned I = 0; I != Packages.size(); ++I)
        if (Name.starts_with(Packages[I].Prefix + ".") &&
            Packages[I].Prefix.size() > PrefixSize) {
          Owner = I;
          PrefixSize = Packages[I].Prefix.size();
        }
    }
    json::Object S{{"package", Owner}, {"name", Name.str()}, {"abi", ABI}};
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
  }

  const Package &P = Packages[Main];
  M.getOrInsertNamedMetadata("goobj.config")->addOperand(P.Config);
  // Library loading and the original import DAG are supplied by the manifest.
  // Putting the union of imports on main would introduce spurious cycles.
  M.getOrInsertNamedMetadata("goobj.imports");
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

static Error optimize(Module &M, StringRef Pipeline, lto::Config &Conf) {
  static std::once_flag TargetsInitialized;
  std::call_once(TargetsInitialized, [] {
    InitializeAllTargetInfos();
    InitializeAllTargets();
    InitializeAllTargetMCs();
  });
  // Validate the test harness override before LTOBackend's fatal-error path.
  ModulePassManager ValidationPipeline;
  PassBuilder PB;
  if (Error E = PB.parsePassPipeline(ValidationPipeline, Pipeline))
    return E;
  Conf.OptPipeline = Pipeline.str();
  // We own M: stop after optimization, then restore GoObj output metadata
  // directly. No second LLVMContext, bitcode round trip, or module clone.
  Conf.PostOptModuleHook = [](unsigned, const Module &) { return false; };
  ModuleSummaryIndex Index(false);
  if (Error E = lto::backend(
          Conf,
          [](unsigned,
             const Twine &) -> Expected<std::unique_ptr<CachedFileStream>> {
            return fail("unexpected FullLTO native code generation");
          },
          1, M, Index, {}))
    return E;
  return Error::success();
}

Error goallc::runLTO(ArrayRef<std::string> Inputs, StringRef OutDir,
                     StringRef Pipeline, bool SaveIR) {
  if (Inputs.empty())
    return fail("LTO requires at least one input");
  lto::Config Conf;
  Conf.ShouldDiscardValueNames = false;
  std::string Diagnostics;
  Conf.DiagHandler = [&](const DiagnosticInfo &DI) {
    if (DI.getSeverity() == DS_Error) {
      raw_string_ostream OS(Diagnostics);
      DiagnosticPrinterRawOStream Printer(OS);
      DI.print(Printer);
      OS << '\n';
    }
  };
  lto::LTOLLVMContext C(Conf);
  std::vector<Package> Packages;
  std::vector<std::unique_ptr<Module>> Modules;
  std::vector<SymbolIdentity> InputSymbols;
  uint64_t NextContentID = 0;
  struct BSSRequirement {
    uint64_t Size = 0;
    bool FunctionAllowed = true;
  };
  StringMap<BSSRequirement> BSSRequirements;
  for (unsigned I = 0; I != Inputs.size(); ++I) {
    SMDiagnostic Diag;
    auto M = parseIRFile(Inputs[I], Diag, C);
    if (!M) {
      std::string Message;
      raw_string_ostream OS(Message);
      Diag.print("LTO", OS);
      return fail(Message);
    }
    if (SaveIR) {
      std::string Dir = (OutDir + "/inputs").str();
      if (std::error_code EC = sys::fs::create_directories(Dir))
        return fail(EC.message());
      std::error_code EC;
      ToolOutputFile IR(Dir + "/" + std::to_string(I) + ".ll", EC,
                        sys::fs::OF_Text);
      if (EC)
        return fail(EC.message());
      M->print(IR.os(), nullptr);
      IR.keep();
    }
    if (!Modules.empty() &&
        (Modules.front()->getTargetTriple() != M->getTargetTriple() ||
         Modules.front()->getDataLayout() != M->getDataLayout()))
      return fail("incompatible target or data layout");
    // Functional lowering belongs to the compiler, before IR serialization.
    // Do not silently accept old bitcode that bypassed CPU/FMV preparation.
    if (M->getNamedMetadata("goallc.cpu.config") &&
        !M->getNamedMetadata("goallc.cpu.fmv.done"))
      return fail("input requires compiler early IR preparation");
    // Linkname access checks use each original reference package in the
    // manifest. Optimization may inline bodies across package boundaries.
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
    for (const MDOperand &Field : P.Config->operands())
      if (!isa<MDString>(Field))
        return fail("invalid GoObj configuration field");
    if (!M->getModuleFlag("goobj.abi"))
      return fail("input requires a goobj.abi module flag");
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
    // LLVM's Error module flag checks the shared goobj.abi contract.
    llvm::append_range(P.ModuleAsm, M->getModuleInlineAsm());
    for (const auto &Fragment : P.ModuleAsm) {
      StringRef Asm = StringRef(Fragment.Asm).trim();
      if (!Asm.empty() &&
          (!Asm.starts_with(".goobj.cgo ") || Asm.contains('\n')))
        return fail("unsupported module assembly in " + P.Prefix);
    }
    M->removeModuleInlineAsm();
    // Package identity and the original import DAG are manifest state.
    // Native debug metadata and ABI module flags merge directly in LLVM.
    M->eraseNamedMetadata(Config);
    if (auto *Imports = M->getNamedMetadata("goobj.imports")) {
      llvm::append_range(P.Imports, Imports->operands());
      M->eraseNamedMetadata(Imports);
    }
    if (auto *CPU = M->getNamedMetadata("goallc.cpu.config")) {
      if (CPU->getNumOperands() != 1)
        return fail("input requires one CPU configuration");
      // Preparation is complete; function attributes carry the target policy.
      M->eraseNamedMetadata(CPU);
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
    SmallVector<GlobalValue *> Retained;
    for (GlobalObject &G : M->global_objects()) {
      if (G.isDeclaration() || G.getName().starts_with("llvm."))
        continue;
      bool HashedFunction =
          isa<Function>(G) && G.getMetadata("goobj.content_addressable");
      if (HashedFunction)
        giveContentName(G, NextContentID);
      if (G.hasLocalLinkage() && !HashedFunction && !clonedConstant(G) &&
          !G.getMetadata("goobj.symbol.index")) {
        if (NextIndex >= UINT32_MAX)
          return fail("too many package symbols");
        Metadata *Index[] = {
            ConstantAsMetadata::get(
                ConstantInt::get(Type::getInt32Ty(C), NextIndex++)),
            ConstantAsMetadata::get(ConstantInt::get(
                Type::getInt16Ty(C),
                GoObj::SymABIstatic))};
        G.setMetadata("goobj.symbol.index", MDNode::get(C, Index));
        G.setMetadata("goobj.symbol.nonpackage", nullptr);
      }
      // Rebase the existing GoObj index into a unique input identity during
      // linking. LLVM keeps the selected definition's attachment (and renames
      // colliding locals); the side table supplies its original package/index.
      // Restore the compiler's index before optimizing or emitting GoObj IR.
      bool Retain = !clonedConstant(G) && !HashedFunction;
      MDNode *Index = G.getMetadata("goobj.symbol.index");
      if (InputSymbols.size() >= UINT32_MAX)
        return fail("too many combined symbols");
      InputSymbols.push_back(
          {I, Index, G.getSection().str()});
      G.setMetadata(
          "goobj.symbol.index",
          MDNode::get(C, ConstantAsMetadata::get(ConstantInt::get(
                             Type::getInt32Ty(C), InputSymbols.size() - 1))));
      if (Retain)
        Retained.push_back(&G);
      if (auto *V = dyn_cast<GlobalVariable>(&G);
          V && !Index && !V->hasLocalLinkage() && V->hasInitializer() &&
          V->getInitializer()->isNullValue()) {
        auto *Flags = V->getMetadata("goobj.symbol.flags");
        bool Linkname =
            Flags && (mdconst::extract<ConstantInt>(Flags->getOperand(1))
                          ->getZExtValue() &
                      (GoObj::SymFlagLinkname | GoSymFlagLinknameStd));
        if (Linkname ||
            (V->getMetadata("goobj.symbol.nonpackage") &&
             (V->getSection() == ".bss" || V->getSection() == ".noptrbss"))) {
          auto &Requirement = BSSRequirements[G.getName()];
          Requirement.Size = std::max(Requirement.Size,
                                      M->getDataLayout()
                                          .getTypeAllocSize(V->getValueType())
                                          .getFixedValue());
          Requirement.FunctionAllowed &= Linkname;
          // LLVM common linkage handles largest size/alignment, strong
          // definitions and linknamed function definitions automatically.
          V->setLinkage(GlobalValue::CommonLinkage);
          V->setSection("");
        }
      }
    }
    appendToCompilerUsed(*M, Retained);
    Packages.push_back(std::move(P));
    M->setModuleIdentifier(Inputs[I]);
    Modules.push_back(std::move(M));
  }
  auto Combined = std::move(Modules.front());
  Linker L(*Combined);
  for (unsigned I = 1; I != Modules.size(); ++I)
    if (L.linkInModule(std::move(Modules[I])))
      return fail(Diagnostics.empty() ? "module linking failed" : Diagnostics);

  for (const auto &R : BSSRequirements) {
    auto *G = Combined->getNamedValue(R.getKey());
    if (isa_and_nonnull<Function>(G)) {
      if (!R.second.FunctionAllowed)
        return fail(
            "BSS declaration resolves to a function without linkname: " +
            R.getKey());
      continue;
    }
    auto *V = dyn_cast_or_null<GlobalVariable>(G);
    if (!V || Combined->getDataLayout().getTypeAllocSize(V->getValueType()) <
                  R.second.Size)
      return fail("BSS declaration exceeds initialized definition: " +
                  R.getKey());
  }
  StringMap<SymbolIdentity> Symbols;
  for (GlobalObject &G : Combined->global_objects()) {
    if (G.isDeclaration() || G.getName().starts_with("llvm."))
      continue;
    auto *Index = G.getMetadata("goobj.symbol.index");
    if (!Index)
      return fail("linked definition lost its Go identity: " + G.getName());
    unsigned ID =
        mdconst::extract<ConstantInt>(Index->getOperand(0))->getZExtValue();
    const SymbolIdentity &Origin = InputSymbols[ID];
    Symbols.try_emplace(G.getName(), Origin);
    G.setMetadata("goobj.symbol.index", Origin.Index);
    G.setMetadata("goobj.import", nullptr);
    if (G.hasCommonLinkage()) {
      G.setLinkage(GlobalValue::ExternalLinkage);
      G.setSection(Origin.Section);
    }
    if (!G.hasLocalLinkage())
      G.setDSOLocal(true);
  }
  if (Error E = optimize(*Combined, Pipeline, Conf))
    return E;
  if (!Diagnostics.empty())
    return fail(Diagnostics);
  if (Error E = checkSymbolIdentities(*Combined, Symbols))
    return E;
  for (GlobalObject &G : Combined->global_objects())
    if (G.hasLocalLinkage() && G.getMetadata("goobj.symbol.index"))
      G.setLinkage(GlobalValue::ExternalLinkage);
  return writeSingleModule(*Combined, Packages, OutDir, SaveIR, Symbols);
}

// The caller owns the returned error string and frees it with free().
extern "C" char *GoALLCLTO(const char *const *Inputs, size_t Count,
                           const char *OutDir, int SaveIR) {
  std::vector<std::string> Paths;
  for (size_t I = 0; I != Count; ++I)
    Paths.emplace_back(Inputs[I]);
  if (Error E = goallc::runLTO(Paths, OutDir, "lto<O2>", SaveIR))
    return strdup(toString(std::move(E)).c_str());
  return nullptr;
}
