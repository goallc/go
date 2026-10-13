// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

#include "llvm/ADT/DenseSet.h"
#include "llvm/ADT/SmallPtrSet.h"
#include "llvm/ADT/StringMap.h"
#include "llvm/BinaryFormat/GoObj.h"
#include "llvm/IR/Constants.h"
#include "llvm/IR/InstIterator.h"
#include "llvm/IR/LLVMContext.h"
#include "llvm/IR/Module.h"
#include "llvm/IR/Verifier.h"
#include "llvm/IRReader/IRReader.h"
#include "llvm/Linker/Linker.h"
#include "llvm/Passes/PassBuilder.h"
#include "llvm/Plugins/PassPlugin.h"
#include "GoALLCLTO.h"
#include "llvm/Bitcode/BitcodeWriter.h"
#include <cstring>
#include <cstdlib>
#include "llvm/Support/FileSystem.h"
#include "llvm/Support/SourceMgr.h"
#include "llvm/Support/ToolOutputFile.h"
#include "llvm/Transforms/Utils/Cloning.h"
#include "llvm/Transforms/Utils/ModuleUtils.h"
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
      return fail("optimization produced a definition without package ownership: " +
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
      return fail("optimization changed retained Go symbol identity: " + S.getKey());
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

Error goallc::runFullLTO(ArrayRef<std::string> Inputs, StringRef OutDir,
                         StringRef Pipeline, bool SaveIR) {
  if (Inputs.empty())
    return fail("FullLTO requires at least one input");
  LLVMContext C;
  std::vector<Package> Packages;
  std::unique_ptr<Module> Combined;
  for (unsigned I = 0; I != Inputs.size(); ++I) {
    SMDiagnostic Diag;
    auto M = parseIRFile(Inputs[I], Diag, C);
    if (!M) {
      std::string Message;
      raw_string_ostream OS(Message);
      Diag.print("FullLTO", OS);
      return fail(Message);
    }
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
      if (G.hasLocalLinkage() && !clonedConstant(G) &&
          !G.getMetadata("goobj.symbol.index")) {
        if (NextIndex >= UINT32_MAX)
          return fail("too many package symbols");
        Metadata *Index[] = {ConstantAsMetadata::get(ConstantInt::get(
                                 Type::getInt32Ty(C), NextIndex++)),
                             ConstantAsMetadata::get(ConstantInt::get(
                                 Type::getInt16Ty(C), GoObj::SymABIstatic))};
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
  if (std::error_code EC = sys::fs::create_directories(OutDir))
    return fail(EC.message());

  for (unsigned I = 0; I != Packages.size(); ++I) {
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
    DenseMap<const GlobalObject *, const GlobalObject *> Originals;
    for (GlobalObject &Original : Combined->global_objects())
      if (auto *Copy = dyn_cast_or_null<GlobalObject>(Map.lookup(&Original)))
        Originals[Copy] = &Original;
    for (GlobalObject &G : M->global_objects()) {
      const GlobalObject *Original = Originals.lookup(&G);
      auto O = Original ? owner(*Original) : std::nullopt;
      if (G.isDeclaration() && O) {
        auto *Index = Original->getMetadata("goobj.symbol.index");
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
          auto *V = dyn_cast<ValueAsMetadata>(E->getOperand(0));
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
extern "C" char *GoALLCFullLTO(const char *const *Inputs, size_t Count,
                               const char *OutDir, int SaveIR) {
  std::vector<std::string> Paths;
  for (size_t I = 0; I != Count; ++I)
    Paths.emplace_back(Inputs[I]);
  if (Error E = goallc::runFullLTO(Paths, OutDir, "lto<O2>", SaveIR))
    return strdup(toString(std::move(E)).c_str());
  return nullptr;
}
