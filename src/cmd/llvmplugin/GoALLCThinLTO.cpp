// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

#include "GoALLCThinLTO.h"
#include "GoALLCThinLTOC.h"
#include "llvm-c/TargetMachine.h"
#include "llvm/ADT/DenseSet.h"
#include "llvm/ADT/MapVector.h"
#include "llvm/ADT/SmallPtrSet.h"
#include "llvm/Analysis/ModuleSummaryAnalysis.h"
#include "llvm/Analysis/ProfileSummaryInfo.h"
#include "llvm/BinaryFormat/GoObj.h"
#include "llvm/Bitcode/BitcodeReader.h"
#include "llvm/Bitcode/BitcodeWriter.h"
#include "llvm/CodeGen/CommandFlags.h"
#include "llvm/IR/Constants.h"
#include "llvm/IR/DebugInfo.h"
#include "llvm/IR/DiagnosticInfo.h"
#include "llvm/IR/DiagnosticPrinter.h"
#include "llvm/IR/Instructions.h"
#include "llvm/IR/LegacyPassManager.h"
#include "llvm/IR/Module.h"
#include "llvm/IR/TrackingMDRef.h"
#include "llvm/IR/Verifier.h"
#include "llvm/LTO/LTOBackend.h"
#include "llvm/MC/TargetRegistry.h"
#include "llvm/Support/FileSystem.h"
#include "llvm/Support/MemoryBuffer.h"
#include "llvm/Support/TargetSelect.h"
#include "llvm/Support/ThreadPool.h"
#include "llvm/Support/ToolOutputFile.h"
#include "llvm/Target/TargetMachine.h"
#include "llvm/Transforms/IPO/FunctionImport.h"
#include "llvm/Transforms/Utils/AssignGUID.h"
#include "llvm/Transforms/Utils/ModuleUtils.h"
#include <cstdlib>
#include <cstring>
#include <mutex>

using namespace llvm;

static Error fail(const Twine &Message) {
  return createStringError(inconvertibleErrorCode(), Message);
}

static Expected<StringRef> packagePrefix(Module &M) {
  auto *Config = M.getNamedMetadata("goobj.config");
  if (!Config || Config->getNumOperands() != 1 ||
      Config->getOperand(0)->getNumOperands() != 5)
    return fail("ThinLTO requires one compiler GoObj configuration");
  auto *Prefix = dyn_cast<MDString>(Config->getOperand(0)->getOperand(1));
  if (!Prefix || Prefix->getString().empty())
    return fail("invalid ThinLTO package prefix");
  if (Prefix->getString() == "crypto/internal/fips140" ||
      Prefix->getString().starts_with("crypto/internal/fips140/") ||
      M.getNamedMetadata("goobj.static_rodata_type"))
    return fail("FIPS packages must use ordinary object compilation");
  if (!M.getModuleFlag("goobj.abi"))
    return fail("ThinLTO input requires a goobj.abi module flag");
  if (M.getNamedMetadata("goallc.cpu.config") &&
      !M.getNamedMetadata("goallc.cpu.fmv.done"))
    return fail("ThinLTO input requires compiler early IR preparation");
  if (M.getNamedMetadata("goobj.dynlink"))
    return fail("ThinLTO does not support dynamic Go linking");
  if (!M.alias_empty() || !M.ifunc_empty())
    return fail("ThinLTO does not support aliases or ifuncs");
  return Prefix->getString();
}

static bool hashed(const GlobalObject &G) {
  return G.getMetadata("goobj.content_hash") ||
         G.getMetadata("goobj.content_addressable");
}

// Native Go symbol resolution remains authoritative. Only import bodies whose
// references can be represented in a different package's ordinary Go object.
// In particular, a content-addressed local has no cross-package export index,
// and a linkname reference must retain its original permission-checking owner.
static bool exportableReference(const Value *V,
                                SmallPtrSetImpl<const Value *> &Seen) {
  if (!Seen.insert(V).second)
    return true;
  if (const auto *G = dyn_cast<GlobalValue>(V)) {
    StringRef Name = G->getName();
    Name.consume_back(GoObj::ABI0SymbolSuffix);
    if (GoObj::getSymbolSuffix(Name) == GoObj::LinknameSymbolSuffix)
      return false;
    if (G->isDeclaration() || Name.starts_with("llvm."))
      return true;
    const auto *GO = dyn_cast<GlobalObject>(G);
    if (!GO || hashed(*GO))
      return false;
    return GO->getMetadata("goobj.symbol.index") || !G->hasLocalLinkage();
  }
  if (const auto *C = dyn_cast<Constant>(V))
    for (const Use &U : C->operands())
      if (!exportableReference(U.get(), Seen))
        return false;
  return true;
}

// Conservative import policy for the current GoObj representation, not a
// ThinLTO requirement. Relax individual restrictions only when Go symbol
// ownership and metadata-only reachability survive the importer.
static bool canImportGoFunction(const Function &F) {
  if (hashed(F) || F.isInterposable() || F.getMetadata("goobj.keep") ||
      F.getMetadata("goobj.marker_relocs") || F.getMetadata("goobj.weak_calls"))
    return false;
  SmallPtrSet<const Value *, 32> Seen;
  for (const BasicBlock &BB : F)
    for (const Instruction &I : BB) {
      // Relocation targets in metadata are not ThinLTO reference edges. Keep
      // these bodies in their owning package until they have an importable
      // representation; an imported null target would lose Go reachability.
      if (I.getMetadata("goobj.marker_reloc"))
        return false;
      for (const Use &U : I.operands())
        if (!exportableReference(U.get(), Seen))
          return false;
    }
  return true;
}

Error goallc::writeThinBitcode(Module &M, raw_ostream &OS) {
  auto Prefix = packagePrefix(M);
  if (!Prefix)
    return Prefix.takeError();
  LLVMContext &C = M.getContext();
  DenseSet<uint32_t> Indices;
  for (GlobalObject &G : M.global_objects()) {
    auto *Index = G.getMetadata("goobj.symbol.index");
    if (!Index)
      continue;
    auto *I = Index->getNumOperands()
                  ? mdconst::dyn_extract<ConstantInt>(Index->getOperand(0))
                  : nullptr;
    if (!I || !I->getType()->isIntegerTy(32) || G.isDeclaration() ||
        !Indices.insert(I->getZExtValue()).second)
      return fail("invalid ThinLTO package index: " + G.getName());
  }
  SmallVector<GlobalValue *> Retained;
  for (GlobalObject &G : M.global_objects()) {
    if (G.isDeclaration() || G.getName().starts_with("llvm."))
      continue;
    if (auto *Index = G.getMetadata("goobj.symbol.index")) {
      unsigned Flags2 = 0;
      if (auto *Flags = G.getMetadata("goobj.symbol.flags"))
        Flags2 = mdconst::extract<ConstantInt>(Flags->getOperand(1))
                     ->getZExtValue() &
                 GoObj::SymFlagUsedInIface;
      Metadata *Ref[] = {MDString::get(C, *Prefix), Index->getOperand(0),
                         ConstantAsMetadata::get(
                             ConstantInt::get(Type::getInt32Ty(C), Flags2))};
      // During ThinLTO, this existing reference attachment also describes how
      // an imported body may refer back to this definition. Remove it from
      // local definitions before the GoObj writer consumes the output.
      G.setMetadata("goobj.import", MDNode::get(C, Ref));
      Retained.push_back(&G);
    } else if (!G.hasLocalLinkage() && !hashed(G)) {
      Retained.push_back(&G);
    }
  }
  // ThinLTO drops references to unimported globals from named metadata.
  // Preserve the exact debug identity as strings plus Go package references,
  // including linknamed functions whose source name differs from the symbol.
  auto *Debug = M.getOrInsertNamedMetadata("goallc.thin.debug.funcs");
  auto Record = [&](DISubprogram *SP, GlobalObject &G) {
    Metadata *Fields[] = {SP, MDString::get(C, G.getName()),
                          G.getMetadata("goobj.import")};
    Debug->addOperand(MDNode::get(C, Fields));
  };
  for (Function &F : M)
    if (auto *SP = F.getSubprogram())
      Record(SP, F);
  if (auto *Funcs = M.getNamedMetadata("goobj.debug.funcs"))
    for (MDNode *Entry : Funcs->operands()) {
      auto *SP = dyn_cast_or_null<DISubprogram>(Entry->getOperand(0));
      auto *G =
          mdconst::dyn_extract_or_null<GlobalObject>(Entry->getOperand(1));
      if (!SP || !G)
        return fail("invalid ThinLTO debug function mapping");
      Record(SP, *G);
    }
  // Keep original package slots and externally visible definitions until the
  // Go linker resolves native objects and Go-only reachability. This is our
  // current resolution policy; ThinLTO itself does not require retaining all.
  appendToCompilerUsed(M, Retained);
  AssignGUIDPass::runOnModule(M);
  ProfileSummaryInfo PSI(M);
  auto Index = buildModuleSummaryIndex(M, nullptr, &PSI);
  for (GlobalObject &G : M.global_objects()) {
    if (G.isDeclaration() || G.getName().starts_with("llvm."))
      continue;
    auto VI = Index.getValueInfo(G.getGUIDOrFallback());
    if (!VI)
      continue;
    if (!isa<Function>(G) || !canImportGoFunction(cast<Function>(G)))
      for (auto &Summary : VI.getSummaryList())
        Summary->setNotEligibleToImport();
  }
  if (verifyModule(M, &errs()))
    return fail("invalid ThinLTO input module");
  WriteBitcodeToFile(M, OS, false, &Index, true);
  return Error::success();
}

// The standard ThinLTO post-link pipeline already eliminates
// available_externally bodies. Only adapt GoObj metadata here: LLVM's importer
// and generic optimizations do not understand package indices or debug symbols.
static Error finishGoObjModule(Module &M, StringRef Prefix,
                               const DenseSet<uint32_t> &ExpectedIndices) {
  DenseSet<uint32_t> Indices;
  SmallVector<StringRef> MetadataNames;
  M.getContext().getMDKindNames(MetadataNames);
  for (GlobalObject &G : M.global_objects()) {
    auto *Ref = G.getMetadata("goobj.import");
    if (G.isDeclaration()) {
      // Imported declarations carry definition attachments from their source.
      SmallVector<std::pair<unsigned, MDNode *>> MD;
      G.getAllMetadata(MD);
      for (const auto &Entry : MD)
        if (MetadataNames[Entry.first].starts_with("goobj.") &&
            MetadataNames[Entry.first] != "goobj.import")
          G.setMetadata(Entry.first, nullptr);
      continue;
    }
    if (Ref) {
      auto *Owner = dyn_cast<MDString>(Ref->getOperand(0));
      if (!Owner || Owner->getString() != Prefix)
        return fail("ThinLTO emitted an imported definition: " + G.getName());
      G.setMetadata("goobj.import", nullptr);
    }
    if (auto *Index = G.getMetadata("goobj.symbol.index")) {
      uint32_t I =
          mdconst::extract<ConstantInt>(Index->getOperand(0))->getZExtValue();
      if (!ExpectedIndices.count(I) || !Indices.insert(I).second)
        return fail("ThinLTO changed package symbol identity: " + G.getName());
    }
  }
  // Rebuild once from the saved identities, including compiler-inlined scopes.
  // Keep only scopes reachable in the optimized module, not old mapping roots.
  if (auto *Funcs = M.getNamedMetadata("goobj.debug.funcs"))
    M.eraseNamedMetadata(Funcs);
  DebugInfoFinder Finder;
  Finder.processModule(M);
  SmallPtrSet<const DISubprogram *, 32> Needed;
  for (auto *SP : Finder.subprograms())
    Needed.insert(SP);
  auto *Funcs = M.getOrInsertNamedMetadata("goobj.debug.funcs");
  if (auto *Debug = M.getNamedMetadata("goallc.thin.debug.funcs")) {
    for (MDNode *Entry : Debug->operands()) {
      auto *SP = cast<DISubprogram>(Entry->getOperand(0));
      if (!Needed.erase(SP))
        continue;
      StringRef Name = cast<MDString>(Entry->getOperand(1))->getString();
      GlobalValue *G = M.getNamedValue(Name);
      if (!G) {
        auto *F = Function::Create(
            FunctionType::get(Type::getVoidTy(M.getContext()), false),
            GlobalValue::ExternalLinkage, Name, M);
        if (auto *Ref = dyn_cast_or_null<MDNode>(Entry->getOperand(2)))
          F->setMetadata("goobj.import", Ref);
        G = F;
      }
      Metadata *Fields[] = {SP, ValueAsMetadata::get(G)};
      Funcs->addOperand(MDNode::get(M.getContext(), Fields));
    }
    M.eraseNamedMetadata(Debug);
  }
  if (Indices.size() != ExpectedIndices.size())
    return fail("ThinLTO removed a retained package symbol");
  if (!M.alias_empty() || !M.ifunc_empty() || verifyModule(M, &errs()))
    return fail("invalid ThinLTO output module");
  return Error::success();
}

Error goallc::runThinLTO(ArrayRef<MemoryBufferRef> Inputs, StringRef OutDir,
                         unsigned Jobs,
                         function_ref<Error(unsigned, Module &)> Consume) {
  if (Inputs.empty() || Jobs == 0)
    return fail("ThinLTO requires inputs and a positive job count");
  static std::once_flag Initialize;
  std::call_once(Initialize, [] {
    InitializeAllTargetInfos();
    InitializeAllTargets();
    InitializeAllTargetMCs();
    InitializeAllAsmParsers();
    InitializeAllAsmPrinters();
  });
  const bool SaveIR = !OutDir.empty();
  if (SaveIR)
    if (std::error_code EC = sys::fs::create_directories(OutDir))
      return fail(EC.message());
  MapVector<StringRef, BitcodeModule> Modules;
  ModuleSummaryIndex Index(false);
  for (MemoryBufferRef Input : Inputs) {
    StringRef Name = Input.getBufferIdentifier();
    if (Name.empty() || Modules.count(Name))
      return fail("ThinLTO requires unique nonempty module identifiers");
    auto BM = lto::findThinLTOModule(Input);
    if (!BM)
      return BM.takeError();
    if (Error E = BM->readSummary(Index, Name))
      return E;
    Modules.insert({Name, *BM});
  }
  DenseSet<GlobalValue::GUID> External;
  for (auto &Entry : Index) {
    auto Summaries = Index.getValueInfo(Entry).getSummaryList();
    for (auto &S : Summaries) {
      if (!GlobalValue::isLocalLinkage(S->linkage()))
        External.insert(Entry.first);
      // A Go linker may coalesce BSS or content-addressed definitions. Keep
      // every input definition/slot and leave that choice to the native loader.
      if (Summaries.size() > 1)
        S->setNotEligibleToImport();
    }
  }
  DenseMap<StringRef, GVSummaryMapTy> Defined;
  Index.collectDefinedGVSummariesPerModule(Defined);
  FunctionImporter::ImportListsTy Imports;
  DenseMap<StringRef, FunctionImporter::ExportSetTy> Exports;
  auto KeepDefinition = [](GlobalValue::GUID, const GlobalValueSummary *) {
    return true;
  };
  ComputeCrossModuleImport(Index, Defined, KeepDefinition, Imports, Exports);
  lto::Config Config;
  Config.ShouldDiscardValueNames = false;
  thinLTOResolvePrevailingInIndex(
      Config, Index, KeepDefinition,
      [](StringRef, GlobalValue::GUID, GlobalValue::LinkageTypes) {}, External);
  thinLTOInternalizeAndPromoteInIndex(
      Index,
      [&](StringRef Module, ValueInfo VI) {
        return External.count(VI.getGUID()) || Exports[Module].count(VI);
      },
      KeepDefinition);
  // Populate maps before workers access them concurrently.
  for (MemoryBufferRef Input : Inputs) {
    Imports[Input.getBufferIdentifier()];
    Defined[Input.getBufferIdentifier()];
  }

  std::vector<std::string> Errors(Inputs.size());
  DefaultThreadPool Pool(hardware_concurrency(Jobs));
  for (unsigned I = 0; I != Inputs.size(); ++I) {
    Pool.async([&, I] {
      auto Run = [&]() -> Error {
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
        StringRef Name = Inputs[I].getBufferIdentifier();
        auto Parsed = Modules.find(Name)->second.parseModule(C);
        if (!Parsed)
          return Parsed.takeError();
        Module &M = **Parsed;
        M.setModuleIdentifier(Name);
        auto Prefix = packagePrefix(M);
        if (!Prefix)
          return Prefix.takeError();
        std::string Owner = Prefix->str();
        DenseSet<uint32_t> ExpectedSlots;
        for (GlobalObject &G : M.global_objects())
          if (auto *MD = G.getMetadata("goobj.symbol.index"))
            ExpectedSlots.insert(
                mdconst::extract<ConstantInt>(MD->getOperand(0))
                    ->getZExtValue());
        // The importer merges arbitrary named metadata. Restore only the
        // output package's configuration; imported debug graphs remain intact.
        // Ordinary module asm, including .goobj.cgo, is not imported by LLVM.
        // The importer's .symver exception is unsupported by the GoObj parser.
        StringMap<SmallVector<MDNode *>> PackageMetadata;
        for (StringRef Name : {"goobj.config", "goobj.imports",
                               "goallc.cpu.config", "goallc.cpu.fmv.done"})
          if (auto *MD = M.getNamedMetadata(Name))
            llvm::append_range(PackageMetadata[Name], MD->operands());
        if (SaveIR) {
          std::error_code EC;
          ToolOutputFile Input((OutDir + "/" + Twine(I) + ".input.bc").str(),
                               EC, sys::fs::OF_None);
          if (EC)
            return fail(EC.message());
          Input.os() << Inputs[I].getBuffer();
          Input.keep();
          ToolOutputFile F((OutDir + "/" + Twine(I) + ".input.ll").str(), EC,
                           sys::fs::OF_Text);
          if (EC)
            return fail(EC.message());
          M.print(F.os(), nullptr);
          F.keep();
        }
        // EliminateAvailableExternally deletes function bodies and their
        // metadata. Snapshot only those imported definitions; declarations and
        // local definitions keep their attachments without our intervention.
        StringMap<TrackingMDNodeRef> ImportRefs;
        Conf.PostImportModuleHook = [&](unsigned, const Module &Imported) {
          for (const GlobalObject &G : Imported.global_objects())
            if (G.hasAvailableExternallyLinkage())
              if (auto *Ref = G.getMetadata("goobj.import"))
                ImportRefs.try_emplace(G.getName(), Ref);
          return true;
        };
        Conf.PostOptModuleHook = [](unsigned, const Module &) { return false; };
        if (Error E = lto::thinBackend(
                Conf, I,
                [](unsigned, const Twine &)
                    -> Expected<std::unique_ptr<CachedFileStream>> {
                  return fail("unexpected ThinLTO native code generation");
                },
                M, Index, Imports.lookup(Name), Defined.find(Name)->second,
                &Modules, false, {}))
          return E;
        if (!Diagnostics.empty())
          return fail(Diagnostics);
        for (StringRef Name : {"goobj.config", "goobj.imports",
                               "goallc.cpu.config", "goallc.cpu.fmv.done"}) {
          if (auto *MD = M.getNamedMetadata(Name))
            M.eraseNamedMetadata(MD);
          for (auto *MD : PackageMetadata[Name])
            M.getOrInsertNamedMetadata(Name)->addOperand(MD);
        }
        for (GlobalObject &G : M.global_objects())
          if (G.isDeclaration())
            if (auto It = ImportRefs.find(G.getName()); It != ImportRefs.end())
              G.setMetadata("goobj.import", It->second.get());
        if (Error E = finishGoObjModule(M, Owner, ExpectedSlots))
          return E;
        if (SaveIR) {
          std::error_code EC;
          ToolOutputFile BC((OutDir + "/" + Twine(I) + ".bc").str(), EC,
                            sys::fs::OF_None);
          if (EC)
            return fail(EC.message());
          WriteBitcodeToFile(M, BC.os());
          BC.keep();
          ToolOutputFile IR((OutDir + "/" + Twine(I) + ".ll").str(), EC,
                            sys::fs::OF_Text);
          if (EC)
            return fail(EC.message());
          M.print(IR.os(), nullptr);
          IR.keep();
        }
        if (Error E = Consume(I, M))
          return E;
        if (!Diagnostics.empty())
          return fail(Diagnostics);
        return Error::success();
      };
      if (Error E = Run())
        Errors[I] = toString(std::move(E));
    });
  }
  Pool.wait();
  for (const std::string &Message : Errors)
    if (!Message.empty())
      return fail(Message);
  return Error::success();
}

extern "C" LLVMErrorRef LLVMGoALLCRunPreCodeGen(LLVMModuleRef,
                                                LLVMTargetMachineRef,
                                                LLVMCodeGenFileType);

static Error emitGoObj(Module &M, bool PIC, SmallVectorImpl<char> &Object) {
  // LLVM's GoObj configuration and target-pass registration are process-global.
  // Keep configuration, pre-codegen and emission in one critical section. Each
  // worker retains its own Module; no bitcode round trip or context transfer.
  static std::mutex CodeGenMutex;
  std::lock_guard<std::mutex> Lock(CodeGenMutex);
  if (Error E = codegen::configureGoObjFromModule(M))
    return E;
  std::string Message;
  const Target *T = TargetRegistry::lookupTarget(M.getTargetTriple(), Message);
  if (!T)
    return fail(Message);
  std::optional<Reloc::Model> Relocation;
  if (PIC)
    Relocation = Reloc::PIC_;
  std::unique_ptr<TargetMachine> TM(T->createTargetMachine(
      M.getTargetTriple(), "", "", TargetOptions(), Relocation, std::nullopt,
      CodeGenOptLevel::Default));
  if (!TM)
    return fail("creating ThinLTO target machine");
  if (Error E = unwrap(LLVMGoALLCRunPreCodeGen(
          wrap(&M), reinterpret_cast<LLVMTargetMachineRef>(TM.get()),
          LLVMObjectFile)))
    return E;
  if (verifyModule(M, &errs()))
    return fail("invalid ThinLTO pre-codegen module");
  M.setDataLayout(TM->createDataLayout());
  raw_svector_ostream OS(Object);
  legacy::PassManager Passes;
  if (TM->addPassesToEmitFile(Passes, OS, nullptr, CodeGenFileType::ObjectFile))
    return fail("ThinLTO target cannot emit Go objects");
  Passes.run(M);
  return Error::success();
}

extern "C" char *GoALLCThinLTOBuffers(const GoALLCLTOInput *Inputs,
                                      size_t Count, const char *Diagnostics,
                                      int PIC, int Jobs,
                                      GoALLCLTOBuffer *Outputs) {
  if (!Count || Jobs <= 0)
    return strdup("ThinLTO requires inputs and a positive job count");
  SmallVector<MemoryBufferRef> Buffers;
  for (size_t I = 0; I != Count; ++I)
    Buffers.emplace_back(StringRef(Inputs[I].data, Inputs[I].size),
                         Inputs[I].name);
  auto Consume = [&](unsigned I, Module &M) -> Error {
    SmallVector<char, 0> Object;
    if (Error E = emitGoObj(M, PIC, Object))
      return E;
    if (Diagnostics[0]) {
      std::error_code EC;
      ToolOutputFile F((Twine(Diagnostics) + "/" + Twine(I) + ".o").str(), EC,
                       sys::fs::OF_None);
      if (EC)
        return fail(EC.message());
      F.os().write(Object.data(), Object.size());
      F.keep();
    }
    auto *Data = static_cast<char *>(malloc(Object.size()));
    if (!Data)
      return fail("allocating ThinLTO object buffer");
    memcpy(Data, Object.data(), Object.size());
    Outputs[I] = {Data, Object.size()};
    return Error::success();
  };
  if (Error E = goallc::runThinLTO(Buffers, Diagnostics, Jobs, Consume))
    return strdup(toString(std::move(E)).c_str());
  return nullptr;
}

extern "C" char *GoALLCWriteThinBitcode(void *Module, char **Data,
                                        size_t *Size) {
  SmallVector<char, 0> Buffer;
  raw_svector_ostream OS(Buffer);
  if (Error E =
          goallc::writeThinBitcode(*static_cast<llvm::Module *>(Module), OS))
    return strdup(toString(std::move(E)).c_str());
  *Data = static_cast<char *>(malloc(Buffer.size()));
  if (!*Data)
    return strdup("allocating ThinLTO bitcode buffer");
  memcpy(*Data, Buffer.data(), Buffer.size());
  *Size = Buffer.size();
  return nullptr;
}
