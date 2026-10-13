// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

#include "GoALLCLTO.h"
#include "llvm/ADT/DenseSet.h"
#include "llvm/ADT/SmallPtrSet.h"
#include "llvm/ADT/StringMap.h"
#include "llvm/Analysis/ModuleSummaryAnalysis.h"
#include "llvm/Analysis/ProfileSummaryInfo.h"
#include "llvm/BinaryFormat/GoObj.h"
#include "llvm/Bitcode/BitcodeReader.h"
#include "llvm/Bitcode/BitcodeWriter.h"
#include "llvm/CodeGen/GoCallingConv.h"
#include "llvm/IR/Constants.h"
#include "llvm/IR/DebugInfoMetadata.h"
#include "llvm/IR/InstIterator.h"
#include "llvm/IR/LLVMContext.h"
#include "llvm/IR/Module.h"
#include "llvm/IR/Verifier.h"
#include "llvm/IRReader/IRReader.h"
#include "llvm/Linker/Linker.h"
#include "llvm/Passes/PassBuilder.h"
#include "llvm/Plugins/PassPlugin.h"
#include "llvm/Support/FileSystem.h"
#include "llvm/Support/SourceMgr.h"
#include "llvm/Support/ToolOutputFile.h"
#include "llvm/Transforms/IPO/FunctionImport.h"
#include "llvm/Transforms/Utils/Cloning.h"
#include "llvm/Transforms/Utils/FunctionImportUtils.h"
#include "llvm/Transforms/Utils/ModuleUtils.h"
#include <cstdlib>
#include <cstring>
#include <memory>
#include <vector>

using namespace llvm;
extern "C" llvm::PassPluginLibraryInfo llvmGetPassPluginInfo();
static constexpr StringLiteral OwnerMD = "goallc.lto.owner";

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
  G.setName(Name + ".goallc.lto." + Prefix +
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

static Error checkPartitionable(Module &M,
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

static Error optimize(Module &M, StringRef Pipeline) {
  LoopAnalysisManager LAM;
  FunctionAnalysisManager FAM;
  CGSCCAnalysisManager CGAM;
  ModuleAnalysisManager MAM;
  PassBuilder PB;
  llvmGetPassPluginInfo().RegisterPassBuilderCallbacks(PB);
  PB.registerModuleAnalyses(MAM);
  PB.registerCGSCCAnalyses(CGAM);
  PB.registerFunctionAnalyses(FAM);
  PB.registerLoopAnalyses(LAM);
  PB.crossRegisterProxies(LAM, FAM, CGAM, MAM);
  ModulePassManager PM;
  if (Error E = PB.parsePassPipeline(PM, Pipeline))
    return fail(toString(std::move(E)));
  PM.run(M, MAM);
  if (verifyModule(M, &errs()))
    return fail("invalid optimized module");
  return Error::success();
}

// Go relation/debug records can refer to definitions outside LLVM's import
// set. The importer nulls ordinary ValueAsMetadata references to these symbols.
// Transport those references by exact IR name and calling convention, then
// restore actual globals in each output partition before GoObj emission. This
// does not turn weak/marker references into LLVM reachability edges.
static Metadata *transportMetadata(Metadata *MD, Module &M, bool Encode,
                                   DenseMap<Metadata *, Metadata *> &Cache) {
  if (!MD)
    return nullptr;
  if (auto It = Cache.find(MD); It != Cache.end())
    return It->second;
  LLVMContext &C = M.getContext();
  constexpr StringLiteral Tag = "goallc.lto.symbol";
  if (Encode)
    if (auto *V = dyn_cast<ValueAsMetadata>(MD))
      if (auto *G = dyn_cast<GlobalValue>(V->getValue())) {
        auto *F = dyn_cast<Function>(G);
        Metadata *Fields[] = {
            MDString::get(C, Tag), MDString::get(C, G->getName()),
            ConstantAsMetadata::get(
                ConstantInt::get(Type::getInt1Ty(C), F != nullptr)),
            ConstantAsMetadata::get(ConstantInt::get(
                Type::getInt32Ty(C), F ? F->getCallingConv() : 0))};
        return Cache[MD] = MDNode::get(C, Fields);
      }
  auto *N = dyn_cast<MDTuple>(MD);
  if (!N)
    return MD; // Keep debug node identities, scopes, and locations intact.
  if (!Encode && N->getNumOperands() == 4)
    if (auto *Marker = dyn_cast_or_null<MDString>(N->getOperand(0).get());
        Marker && Marker->getString() == Tag) {
      StringRef Name = cast<MDString>(N->getOperand(1))->getString();
      GlobalValue *G = M.getNamedValue(Name);
      if (!G) {
        bool IsFunction =
            mdconst::extract<ConstantInt>(N->getOperand(2))->isOne();
        if (IsFunction) {
          auto *F =
              Function::Create(FunctionType::get(Type::getVoidTy(C), false),
                               GlobalValue::ExternalLinkage, Name, M);
          F->setCallingConv(
              mdconst::extract<ConstantInt>(N->getOperand(3))->getZExtValue());
          G = F;
        } else {
          G = new GlobalVariable(M, Type::getInt8Ty(C), false,
                                 GlobalValue::ExternalLinkage, nullptr, Name);
        }
      }
      return Cache[MD] = ValueAsMetadata::get(G);
    }
  SmallVector<Metadata *> Ops;
  for (const MDOperand &Op : N->operands())
    Ops.push_back(transportMetadata(Op.get(), M, Encode, Cache));
  return Cache[MD] = MDNode::get(C, Ops);
}

static void transportPackageMetadata(Module &M, bool Encode) {
  DenseMap<Metadata *, Metadata *> Cache;
  for (NamedMDNode &N : M.named_metadata()) {
    if (!N.getName().starts_with("goobj."))
      continue;
    SmallVector<MDNode *> Records;
    for (MDNode *E : N.operands())
      Records.push_back(cast<MDNode>(transportMetadata(E, M, Encode, Cache)));
    N.clearOperands();
    for (MDNode *E : Records)
      N.addOperand(E);
  }
  // These instruction records deliberately survive inlining; they must keep
  // their target even when that target is not part of the function import set.
  for (Function &F : M)
    for (Instruction &I : instructions(F))
      if (auto *MD = I.getMetadata("goobj.marker_reloc"))
        I.setMetadata("goobj.marker_reloc",
                      cast<MDNode>(transportMetadata(MD, M, Encode, Cache)));
}

// Build and merge summary indexes, then use LLVM's cost-based import planner.
// Each backend sees its own module plus selected imported definitions. There
// is no whole-program IR module in this mode. Definitions are conservatively
// live because ordinary Go assembly can reference them outside the IR graph.
static Error importThinModules(std::vector<std::unique_ptr<Module>> &Modules,
                               StringRef OutDir, bool SaveIR) {
  ModuleSummaryIndex Index(false);
  StringMap<SmallVector<char, 0>> Bitcode;
  for (auto &M : Modules) {
    // Go content hashes guarantee identical definitions across packages.
    // Express that guarantee to LLVM so these immutable values are importable.
    for (GlobalVariable &G : M->globals())
      if (G.hasInitializer() && clonedConstant(G) &&
          G.getMetadata("goobj.content_hash"))
        G.setLinkage(GlobalValue::WeakODRLinkage);
    if (Error E = optimize(*M, "assign-guid"))
      return E;
    // LLVM 23's summary builder assumes every direct call is call/invoke,
    // while llvm.go.defer.edge is a callbr. Summarize those edges as opaque
    // calls on a temporary clone, retaining the CFG and conservative effects.
    // Restore the original callee before serializing: the importer and every
    // optimization/codegen pass always receive the actual defer intrinsic.
    auto SummaryModule = CloneModule(*M);
    SmallVector<std::pair<CallBrInst *, Value *>> DeferEdges;
    for (Function &F : *SummaryModule)
      for (Instruction &I : instructions(F))
        if (auto *CB = dyn_cast<CallBrInst>(&I))
          if (Function *Callee = CB->getCalledFunction()) {
            if (Callee->getName() != "llvm.go.defer.edge")
              return fail("unsupported direct callbr in ThinLTO summary");
            DeferEdges.emplace_back(CB, Callee);
            CB->setCalledOperand(
                ConstantPointerNull::get(cast<PointerType>(Callee->getType())));
          }
    ProfileSummaryInfo PSI(*SummaryModule);
    auto Summary = buildModuleSummaryIndex(*SummaryModule, nullptr, &PSI);
    for (auto [CB, Callee] : DeferEdges)
      CB->setCalledOperand(Callee);
    auto &Bytes = Bitcode[M->getModuleIdentifier()];
    raw_svector_ostream OS(Bytes);
    WriteBitcodeToFile(*SummaryModule, OS, false, &Summary, true);
    if (Error E = readModuleSummaryIndex(
            MemoryBufferRef(StringRef(Bytes.data(), Bytes.size()),
                            M->getModuleIdentifier()),
            Index))
      return E;
  }
  for (auto &Entry : Index)
    for (auto &Summary : Index.getValueInfo(Entry.first).getSummaryList())
      Summary->setLive(true);
  DenseMap<StringRef, GVSummaryMapTy> Defined;
  Index.collectDefinedGVSummariesPerModule(Defined);
  FunctionImporter::ImportListsTy Imports(Modules.size());
  DenseMap<StringRef, FunctionImporter::ExportSetTy> Exports;
  auto Prevailing = [&](GlobalValue::GUID GUID, const GlobalValueSummary *S) {
    return Index.getValueInfo(GUID).getSummaryList().front().get() == S;
  };
  ComputeCrossModuleImport(Index, Defined, Prevailing, Imports, Exports);
  // Go content-addressed constants have no named linker definition. An
  // imported body that references one must carry its initializer, even when
  // LLVM's ordinary global-import heuristic would leave a declaration.
  DenseMap<GlobalValue::GUID, std::pair<StringRef, StringRef>> Constants;
  for (auto &M : Modules)
    for (GlobalVariable &G : M->globals())
      if (G.hasInitializer() && clonedConstant(G) &&
          G.getMetadata("goobj.content_hash"))
        Constants.try_emplace(G.getGUID(), M->getModuleIdentifier(),
                              G.getName());
  for (auto &M : Modules) {
    auto &ImportList = Imports[M->getModuleIdentifier()];
    SmallVector<const GlobalValueSummary *> Work;
    for (auto &Entry : Defined.lookup(M->getModuleIdentifier()))
      Work.push_back(Entry.second);
    for (auto [From, GUID, Kind] : ImportList)
      if (Kind == GlobalValueSummary::Definition)
        Work.push_back(Index.findSummaryInModule(GUID, From));
    SmallPtrSet<const GlobalValueSummary *, 32> Seen;
    while (!Work.empty()) {
      const auto *S = Work.pop_back_val();
      if (!S || !Seen.insert(S).second)
        continue;
      for (ValueInfo Ref : S->refs()) {
        auto It = Constants.find(Ref.getGUID());
        if (It == Constants.end())
          continue;
        auto [From, Name] = It->second;
        if (auto *G = M->getNamedGlobal(Name); G && G->hasInitializer())
          continue;
        ImportList.addDefinition(From, Ref.getGUID());
        Work.push_back(Index.findSummaryInModule(Ref, From));
      }
    }
  }

  if (SaveIR) {
    if (std::error_code EC = sys::fs::create_directories(OutDir))
      return fail(EC.message());
    std::error_code EC;
    ToolOutputFile Summary((OutDir + "/thin.index.bc").str(), EC,
                           sys::fs::OF_None);
    if (EC)
      return fail(EC.message());
    writeIndexToFile(Index, Summary.os());
    Summary.keep();
  }
  for (auto &M : Modules) {
    auto Loader = [&](StringRef Name) -> Expected<std::unique_ptr<Module>> {
      auto It = Bitcode.find(Name);
      if (It == Bitcode.end())
        return fail("unknown ThinLTO module: " + Name);
      auto &Bytes = It->second;
      return parseBitcodeFile(
          MemoryBufferRef(StringRef(Bytes.data(), Bytes.size()), Name),
          M->getContext());
    };
    renameModuleForThinLTO(*M, Index, false);
    FunctionImporter Importer(Index, Loader, false);
    auto Imported =
        Importer.importFunctions(*M, Imports.lookup(M->getModuleIdentifier()));
    if (!Imported)
      return Imported.takeError();
    // Imported definitions may inherit destination declaration metadata.
    for (GlobalObject &G : M->global_objects())
      if (!G.isDeclaration()) {
        G.setMetadata("goobj.import", nullptr);
        if (clonedConstant(G) && G.hasAvailableExternallyLinkage())
          G.setLinkage(GlobalValue::WeakAnyLinkage);
      }
  }
  return Error::success();
}

Error goallc::runLTO(ArrayRef<std::string> Inputs, StringRef OutDir,
                     StringRef Pipeline, bool SaveIR, bool Thin) {
  if (Inputs.empty())
    return fail("LTO requires at least one input");
  LLVMContext C;
  std::vector<Package> Packages;
  std::unique_ptr<Module> Combined;
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
    if (Error E = optimize(*M, "goallc-cpu-features"))
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
  if (!Thin) {
    for (auto &Input : Modules) {
      auto M = std::move(Input);
      if (!Combined)
        Combined = std::move(M);
      else {
        if (Combined->getTargetTriple() != M->getTargetTriple() ||
            Combined->getDataLayout() != M->getDataLayout())
          return fail("incompatible target or data layout");
        for (GlobalVariable &G : M->globals()) {
          auto *Old = Combined->getNamedGlobal(G.getName());
          auto BSS = [](GlobalVariable &V) {
            return V.hasInitializer() && V.getInitializer()->isNullValue() &&
                   V.getMetadata("goobj.symbol.nonpackage") &&
                   (V.getSection() == ".bss" || V.getSection() == ".noptrbss");
          };
          // Go named BSS includes linkname declarations. Content wins over a
          // declaration; among BSS declarations the largest allocation wins.
          // Growing an initialized definition is deliberately not inferred.
          if (Old && G.hasInitializer() && Old->hasInitializer() &&
              (BSS(G) || BSS(*Old)) && !G.getMetadata("goobj.symbol.index") &&
              !Old->getMetadata("goobj.symbol.index")) {
            const DataLayout &DL = M->getDataLayout();
            auto GS = DL.getTypeAllocSize(G.getValueType());
            auto OS = DL.getTypeAllocSize(Old->getValueType());
            bool NewWins = BSS(*Old) && (!BSS(G) || GS > OS);
            GlobalVariable &Winner = NewWins ? G : *Old;
            GlobalVariable &Loser = NewWins ? *Old : G;
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
          }
        }
        if (Linker::linkModules(*Combined, std::move(M)))
          return fail("module linking failed");
      }
    }
  }
  if (!Thin) {
    // A declaration's import metadata is no longer valid once resolved in IR.
    for (GlobalObject &G : Combined->global_objects())
      if (!G.isDeclaration())
        G.setMetadata("goobj.import", nullptr);

    StringMap<SymbolIdentity> Symbols;
    for (GlobalObject &G : Combined->global_objects()) {
      if (G.isDeclaration() || G.getName().starts_with("llvm."))
        continue;
      if (auto *Index = G.getMetadata("goobj.symbol.index"))
        Symbols.try_emplace(G.getName(), SymbolIdentity{*owner(G), Index});
    }
    if (Error E = optimize(*Combined, Pipeline))
      return E;
    if (Error E = checkPartitionable(*Combined, Symbols))
      return E;
    // Imported inlined bodies can refer to their package's static data. Go's
    // indexed relocation still identifies the original static definition;
    // LLVM linkage must permit a declaration in a different output partition.
    for (GlobalObject &G : Combined->global_objects())
      if (G.hasLocalLinkage() && G.getMetadata("goobj.symbol.index"))
        G.setLinkage(GlobalValue::ExternalLinkage);
  }
  StringMap<SymbolIdentity> ThinSymbols;
  if (Thin) {
    // Give local definitions unique IR names during importing. Restore their
    // original names before GoObj emission so assembly keeps the same symbols;
    // cross-package declarations resolve by package/index, not this IR name.
    StringMap<std::string> Promoted;
    StringMap<unsigned> NameCounts;
    for (auto &M : Modules)
      for (GlobalObject &G : M->global_objects())
        if (!G.isDeclaration() && G.getMetadata("goobj.symbol.index"))
          ++NameCounts[G.getName()];
    for (unsigned I = 0; I != Modules.size(); ++I) {
      for (GlobalObject &G : Modules[I]->global_objects()) {
        if (G.isDeclaration())
          continue;
        if (auto *Index = G.getMetadata("goobj.symbol.index")) {
          if (G.hasLocalLinkage() && NameCounts[G.getName()] > 1) {

            std::string Key = Packages[I].Prefix + ":" +
                              std::to_string(mdconst::extract<ConstantInt>(
                                                 Index->getOperand(0))
                                                 ->getZExtValue());
            givePackageName(G, Packages[I].Prefix);
            G.setLinkage(GlobalValue::ExternalLinkage);
            Promoted[Key] = G.getName().str();
          }
          if (G.hasLocalLinkage())
            G.setLinkage(GlobalValue::ExternalLinkage);
          if (!ThinSymbols.try_emplace(G.getName(), SymbolIdentity{I, Index})
                   .second)
            return fail("duplicate indexed symbol across ThinLTO packages: " +
                        G.getName());
        }
      }
    }
    for (auto &M : Modules)
      for (GlobalObject &G : M->global_objects())
        if (G.isDeclaration())
          if (auto *Import = G.getMetadata("goobj.import")) {
            std::string Key =
                cast<MDString>(Import->getOperand(0))->getString().str() + ":" +
                std::to_string(
                    mdconst::extract<ConstantInt>(Import->getOperand(1))
                        ->getZExtValue());
            auto It = Promoted.find(Key);
            if (It != Promoted.end())
              G.setName(It->second);
          }
    for (auto &M : Modules)
      transportPackageMetadata(*M, true);
    if (Error E = importThinModules(Modules, OutDir, SaveIR))
      return E;
  }
  if (std::error_code EC = sys::fs::create_directories(OutDir))
    return fail(EC.message());

  for (unsigned I = 0; I != Packages.size(); ++I) {
    if (Thin) {
      Combined = std::move(Modules[I]);
      StringMap<SymbolIdentity> Symbols;
      for (auto &S : ThinSymbols)
        if (S.second.Owner == I)
          Symbols.try_emplace(S.getKey(), S.second);
      if (Error E = optimize(*Combined, Pipeline))
        return E;
      if (Error E = checkPartitionable(*Combined, Symbols))
        return E;
    }
    ValueToValueMapTy Map;
    auto M = CloneModule(*Combined, Map, [&](const GlobalValue *G) {
      if (G->getName().starts_with("llvm.") || clonedConstant(*G))
        return true;
      auto *O = dyn_cast<GlobalObject>(G);
      return O && owner(*O) == I;
    });
    removeFromUsedLists(*M, [](Constant *C) {
      auto *G = dyn_cast<GlobalValue>(C->stripPointerCasts());
      return G && G->isDeclaration();
    });
    M->setModuleIdentifier(Inputs[I]);
    M->setModuleInlineAsm(Packages[I].ModuleAsm);
    for (const auto &S : Packages[I].Settings) {
      if (auto *Old = M->getNamedMetadata(S.getKey()))
        M->eraseNamedMetadata(Old);
      auto *N = M->getOrInsertNamedMetadata(S.getKey());
      for (MDNode *E : S.second)
        N->addOperand(E);
    }
    if (Thin)
      transportPackageMetadata(*M, false);
    // A no-DWARF package can import/in-line bodies that carry variable
    // records. Keep source locations for pcln/tracebacks, but honor its
    // original DWARF setting instead of emitting foreign variable records.
    auto *DebugConfig = M->getNamedMetadata("goobj.debug.config");
    if (!DebugConfig || DebugConfig->getOperand(0)->getNumOperands() == 1)
      if (auto *Vars = M->getNamedMetadata("goobj.debug.vars"))
        M->eraseNamedMetadata(Vars);
    DenseMap<const GlobalObject *, const GlobalObject *> Originals;
    for (GlobalObject &Original : Combined->global_objects())
      if (auto *Copy = dyn_cast_or_null<GlobalObject>(Map.lookup(&Original)))
        Originals[Copy] = &Original;
    for (GlobalObject &G : M->global_objects()) {
      const GlobalObject *Original = Originals.lookup(&G);
      auto O = Original ? owner(*Original) : std::nullopt;
      auto *Index =
          Original ? Original->getMetadata("goobj.symbol.index") : nullptr;
      if (Thin) {
        auto It = ThinSymbols.find(G.getName());
        if (It != ThinSymbols.end()) {
          O = It->second.Owner;
          Index = It->second.Index;
        }
      }
      if (G.isDeclaration() && O) {
        SmallVector<std::pair<unsigned, MDNode *>> MDs;
        G.getAllMetadata(MDs);
        for (auto [Kind, N] : MDs)
          G.setMetadata(Kind, nullptr);
        if (Index) {
          Metadata *Ref[] = {
              MDString::get(C, Packages[*O].Prefix), Index->getOperand(0),
              ConstantAsMetadata::get(ConstantInt::get(Type::getInt8Ty(C), 0))};
          G.setMetadata("goobj.import", MDNode::get(C, Ref));
        }
      }
      G.setMetadata(OwnerMD, nullptr);
      if (auto *Name = G.getMetadata("goallc.lto.name")) {
        G.setName(cast<MDString>(Name->getOperand(0))->getString());
        G.setMetadata("goallc.lto.name", nullptr);
      }
    }
    // Inlining introduces references to transitive dependencies. Preserve the
    // original import DAG instead of adding every combined package to every
    // partition (which would create import cycles).
    {
      auto *N = M->getOrInsertNamedMetadata("goobj.imports");
      StringMap<MDNode *> Unique;
      SmallVector<MDNode *> Work(N->operands());
      while (!Work.empty()) {
        MDNode *E = Work.pop_back_val();
        StringRef Path = cast<MDString>(E->getOperand(0))->getString();
        auto [It, Inserted] = Unique.try_emplace(Path, E);
        if (!Inserted && It->second != E)
          return fail("inconsistent import fingerprints");
        if (!Inserted)
          continue;
        StringRef Prefix = cast<MDString>(E->getOperand(1))->getString();
        if (Prefix == Packages[I].Prefix)
          return fail("cyclic package imports");
        for (Package &P : Packages)
          if (P.Prefix == Prefix)
            llvm::append_range(Work, P.Settings["goobj.imports"]);
      }
      N->clearOperands();
      SmallVector<StringRef> Names;
      for (auto &E : Unique)
        Names.push_back(E.getKey());
      llvm::sort(Names);
      for (StringRef Name : Names)
        N->addOperand(Unique[Name]);
    }
    // Relation records are owned by their source definition.
    for (StringRef Name : {"goobj.keep", "goobj.gotype", "goobj.marker_relocs",
                           "goobj.weak_calls"}) {
      if (auto *N = M->getNamedMetadata(Name)) {
        SmallVector<MDNode *> Keep;
        SmallPtrSet<MDNode *, 16> Seen;
        for (MDNode *E : N->operands()) {
          auto *V = dyn_cast_or_null<ValueAsMetadata>(E->getOperand(0).get());
          auto *G = V ? dyn_cast<GlobalValue>(V->getValue()) : nullptr;
          if (Name == "goobj.gotype" && G) {
            auto *Original = Originals.lookup(dyn_cast<GlobalObject>(G));
            unsigned RecordOwner =
                mdconst::extract<ConstantInt>(E->getOperand(2))->getZExtValue();
            if (!Original || owner(*Original) != RecordOwner)
              continue;
            Metadata *Fields[] = {E->getOperand(0), E->getOperand(1)};
            E = MDNode::get(C, Fields);
          }
          if (G && !G->isDeclaration() && Seen.insert(E).second)
            Keep.push_back(E);
        }
        N->clearOperands();
        for (MDNode *E : Keep)
          N->addOperand(E);
      }
    }
    for (StructType *T : M->getIdentifiedStructTypes())
      if (T->hasName() && (T->getName() == "go.abi.pad" ||
                           T->getName().starts_with("go.abi.pad."))) {
        if (auto *Old = StructType::getTypeByName(C, "go.abi.pad");
            Old && Old != T)
          Old->setName("go.abi.pad.saved");
        T->setName("go.abi.pad");
      }
    if (verifyModule(*M, &errs()))
      return fail("invalid package partition");
    std::error_code EC;
    ToolOutputFile Out((OutDir + "/" + std::to_string(I) + ".bc").str(), EC,
                       sys::fs::OF_None);
    if (EC)
      return fail(EC.message());
    WriteBitcodeToFile(*M, Out.os());
    Out.os().flush();
    if (Out.os().has_error())
      return fail("cannot write package bitcode");
    Out.keep();
    if (SaveIR) {
      ToolOutputFile IR((OutDir + "/" + std::to_string(I) + ".ll").str(), EC,
                        sys::fs::OF_Text);
      if (EC)
        return fail(EC.message());
      M->print(IR.os(), nullptr);
      IR.keep();
    }
  }
  return Error::success();
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
