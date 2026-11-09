// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

#include "GoALLCThinLTO.h"
#include "GoALLCPreCodeGen.h"
#include "GoALLCThinLTOC.h"
#include "llvm/ADT/DenseSet.h"
#include "llvm/ADT/SmallPtrSet.h"
#include "llvm/ADT/StringExtras.h"
#include "llvm/Analysis/ModuleSummaryAnalysis.h"
#include "llvm/Analysis/ProfileSummaryInfo.h"
#include "llvm/BinaryFormat/GoObj.h"
#include "llvm/Bitcode/BitcodeReader.h"
#include "llvm/Bitcode/BitcodeWriter.h"
#include "llvm/IR/Constants.h"
#include "llvm/IR/DiagnosticInfo.h"
#include "llvm/IR/DiagnosticPrinter.h"
#include "llvm/IR/Instructions.h"
#include "llvm/IR/Module.h"
#include "llvm/IR/Verifier.h"
#include "llvm/LTO/LTO.h"
#include "llvm/LTO/LTOBackend.h"
#include "llvm/Support/FileSystem.h"
#include "llvm/Support/MemoryBuffer.h"
#include "llvm/Support/TargetSelect.h"
#include "llvm/Support/Threading.h"
#include "llvm/Support/ToolOutputFile.h"
#include "llvm/Transforms/Utils/AssignGUID.h"
#include "llvm/Transforms/Utils/ModuleUtils.h"
#include <mutex>

using namespace llvm;

static Error fail(const Twine &Message) {
  return createStringError(inconvertibleErrorCode(), Message);
}

static Expected<StringRef> packagePrefix(const Module &M) {
  auto *Config = M.getNamedMetadata("goobj.config");
  if (!Config || Config->getNumOperands() != 1 ||
      Config->getOperand(0)->getNumOperands() != 5)
    return fail("ThinLTO requires one compiler GoObj configuration");
  auto *Prefix = dyn_cast<MDString>(Config->getOperand(0)->getOperand(1));
  if (!Prefix || Prefix->getString().empty())
    return fail("invalid ThinLTO package prefix");
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
  if (F.getMetadata("goallc.lto.noimport") || hashed(F) || F.isInterposable() ||
      F.getMetadata("goobj.keep") || F.getMetadata("goobj.marker_relocs") ||
      F.getMetadata("goobj.weak_calls"))
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
  // Function::deleteBody drops metadata when ThinLTO eliminates an imported
  // body, but preserves function attributes. Keep reference identity there so
  // declarations need no post-import snapshot or reconstruction. Global
  // variable metadata already survives initializer removal.
  for (Function &F : M)
    if (auto *Ref = F.getMetadata("goobj.import")) {
      auto *Owner = Ref->getNumOperands() == 3
                        ? dyn_cast<MDString>(Ref->getOperand(0))
                        : nullptr;
      auto *Index = Owner
                        ? mdconst::dyn_extract<ConstantInt>(Ref->getOperand(1))
                        : nullptr;
      auto *Flags = Owner
                        ? mdconst::dyn_extract<ConstantInt>(Ref->getOperand(2))
                        : nullptr;
      if (!Owner || Owner->getString().empty() || !Index ||
          Index->getValue().ugt(UINT32_MAX) || !Flags ||
          Flags->getValue().ugt(UINT8_MAX))
        return fail("invalid ThinLTO function reference: " + F.getName());
      F.addFnAttr("goobj.import.package", Owner->getString());
      F.addFnAttr("goobj.import.index", utostr(Index->getZExtValue()));
      F.addFnAttr("goobj.import.flags2", utostr(Flags->getZExtValue()));
      F.setMetadata("goobj.import", nullptr);
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
  for (Function &F : M)
    F.setMetadata("goallc.lto.noimport", nullptr);
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
    if (auto *F = dyn_cast<Function>(&G)) {
      Attribute Owner = F->getFnAttribute("goobj.import.package");
      if (Owner.isValid()) {
        if (Owner.getValueAsString() != Prefix)
          return fail("ThinLTO emitted an imported definition: " +
                      F->getName());
        F->removeFnAttr("goobj.import.package");
        F->removeFnAttr("goobj.import.index");
        F->removeFnAttr("goobj.import.flags2");
      }
    }
    if (auto *Index = G.getMetadata("goobj.symbol.index")) {
      uint32_t I =
          mdconst::extract<ConstantInt>(Index->getOperand(0))->getZExtValue();
      if (!ExpectedIndices.count(I) || !Indices.insert(I).second)
        return fail("ThinLTO changed package symbol identity: " + G.getName());
    }
  }
  if (Indices.size() != ExpectedIndices.size())
    return fail("ThinLTO removed a retained package symbol");
  if (!M.alias_empty() || !M.ifunc_empty() || verifyModule(M, &errs()))
    return fail("invalid ThinLTO output module");
  return Error::success();
}

Error goallc::runThinLTO(ArrayRef<MemoryBufferRef> Inputs, StringRef OutDir,
                         unsigned Jobs, bool PIC, AddStreamFn AddStream) {
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
  // LLVM owns index construction, import planning and backend scheduling.
  // Normalize Go-owned duplicate definitions before supplying resolutions.
  std::vector<std::unique_ptr<lto::InputFile>> Files;
  StringMap<unsigned> Definitions;
  StringSet<> AllIRNames;
  StringSet<> Names;
  for (MemoryBufferRef Input : Inputs) {
    StringRef Name = Input.getBufferIdentifier();
    if (Name.empty() || !Names.insert(Name).second)
      return fail("ThinLTO requires unique nonempty module identifiers");
    auto File = lto::InputFile::create(Input);
    if (!File)
      return File.takeError();
    auto BM = lto::findThinLTOModule(Input);
    if (!BM)
      return BM.takeError();
    for (const auto &Sym : (*File)->symbols()) {
      AllIRNames.insert(Sym.getIRName());
      if (!Sym.isUndefined())
        ++Definitions[Sym.getName()];
    }
    Files.push_back(std::move(*File));
  }
  // Go's native loader, not LLVM name resolution, owns
  // duplicate package slots, BSS replacement and content-addressed definitions.
  // Give each such definition a distinct LLVM identity for the LTO interval.
  // Do not import functions that refer to these unresolved native identities.
  std::vector<StringMap<std::string>> OriginalNames(Inputs.size());
  StringSet<> TemporaryNames;
  std::vector<std::unique_ptr<MemoryBuffer>> Normalized;
  StringSet<> AmbiguousIRNames;
  for (auto &File : Files)
    for (const auto &Sym : File->symbols())
      if (!Sym.getIRName().empty() && Definitions.lookup(Sym.getName()) > 1)
        AmbiguousIRNames.insert(Sym.getIRName());
  for (unsigned I = 0; !AmbiguousIRNames.empty() && I < Files.size(); ++I) {
    LLVMContext C;
    auto Parsed = Files[I]->getSingleBitcodeModule().parseModule(C);
    if (!Parsed)
      return Parsed.takeError();
    Module &M = **Parsed;
    auto UsesAmbiguous = [&](const Value *V, auto &&Self,
                             SmallPtrSetImpl<const Value *> &Seen) -> bool {
      if (!Seen.insert(V).second)
        return false;
      if (auto *GV = dyn_cast<GlobalValue>(V))
        return AmbiguousIRNames.count(GV->getName());
      if (auto *K = dyn_cast<Constant>(V))
        for (const Use &U : K->operands())
          if (Self(U.get(), Self, Seen))
            return true;
      return false;
    };
    for (Function &F : M) {
      SmallPtrSet<const Value *, 32> Seen;
      bool Unresolved = AmbiguousIRNames.count(F.getName());
      for (BasicBlock &BB : F)
        for (Instruction &Inst : BB)
          for (const Use &U : Inst.operands())
            Unresolved |= UsesAmbiguous(U.get(), UsesAmbiguous, Seen);
      if (Unresolved)
        F.setMetadata("goallc.lto.noimport", MDNode::get(C, {}));
    }
    unsigned N = 0;
    for (GlobalObject &G : M.global_objects()) {
      if (G.isDeclaration() || !AmbiguousIRNames.count(G.getName()))
        continue;
      std::string Temporary;
      do {
        Temporary = "__goallc_lto." + utostr(I) + "." + utostr(N++);
      } while (M.getNamedValue(Temporary) || AllIRNames.count(Temporary));
      AllIRNames.insert(Temporary);
      TemporaryNames.insert(Temporary);
      OriginalNames[I][Temporary] = G.getName().str();
      G.setName(Temporary);
      G.setMetadata("guid", nullptr);
    }
    SmallVector<char, 0> Bytes;
    raw_svector_ostream OS(Bytes);
    if (Error E = goallc::writeThinBitcode(M, OS))
      return E;
    Normalized.push_back(
        MemoryBuffer::getMemBufferCopy(StringRef(Bytes.data(), Bytes.size()),
                                       Inputs[I].getBufferIdentifier()));
    auto File = lto::InputFile::create(Normalized.back()->getMemBufferRef());
    if (!File)
      return File.takeError();
    Files[I] = std::move(*File);
  }
  struct PackageState {
    std::string Owner;
    DenseSet<uint32_t> Slots;
    StringMap<SmallVector<MDNode *>> Metadata;
    std::string Error;
    bool Finished = false;
  };
  // Task zero is reserved by LTO for its regular-LTO module. ThinLTO tasks
  // follow in input order, independently of backend completion order.
  std::vector<PackageState> States(Inputs.size() + 1);
  static constexpr StringLiteral PackageMetadataNames[] = {
      "goobj.config", "goobj.imports", "goobj.static_rodata_type",
      "goallc.cpu.config", "goallc.cpu.fmv.done"};
  auto Save = [&](unsigned Task, const Module &M, StringRef Suffix) -> Error {
    if (!SaveIR)
      return Error::success();
    std::string Base = (OutDir + "/" + Twine(Task - 1) + Suffix).str();
    std::error_code EC;
    ToolOutputFile BC(Base + ".bc", EC, sys::fs::OF_None);
    if (EC)
      return fail(EC.message());
    if (Suffix == ".input")
      BC.os() << (Normalized.empty() ? Inputs[Task - 1].getBuffer()
                                     : Normalized[Task - 1]->getBuffer());
    else
      WriteBitcodeToFile(M, BC.os());
    BC.keep();
    ToolOutputFile IR(Base + ".ll", EC, sys::fs::OF_Text);
    if (EC)
      return fail(EC.message());
    M.print(IR.os(), nullptr);
    IR.keep();
    return Error::success();
  };
  lto::Config Conf;
  Conf.ShouldDiscardValueNames = false;
  Conf.RelocModel =
      PIC ? std::optional<Reloc::Model>(Reloc::PIC_) : std::nullopt;
  std::mutex DiagnosticsMutex;
  std::string Diagnostics;
  Conf.DiagHandler = [&](const DiagnosticInfo &DI) {
    if (DI.getSeverity() == DS_Error) {
      std::lock_guard<std::mutex> Lock(DiagnosticsMutex);
      raw_string_ostream OS(Diagnostics);
      DiagnosticPrinterRawOStream Printer(OS);
      DI.print(Printer);
      OS << '\n';
    }
  };
  Conf.PreOptModuleHook = [&](unsigned Task, const Module &M) {
    if (!Task || Task >= States.size())
      return false;
    auto &S = States[Task];
    auto Prefix = packagePrefix(M);
    if (!Prefix) {
      S.Error = toString(Prefix.takeError());
      return false;
    }
    S.Owner = Prefix->str();
    for (const GlobalObject &G : M.global_objects())
      if (auto *MD = G.getMetadata("goobj.symbol.index"))
        S.Slots.insert(
            mdconst::extract<ConstantInt>(MD->getOperand(0))->getZExtValue());
    for (StringRef Name : PackageMetadataNames)
      if (auto *MD = M.getNamedMetadata(Name))
        llvm::append_range(S.Metadata[Name], MD->operands());
    if (Error E = Save(Task, M, ".input")) {
      S.Error = toString(std::move(E));
      return false;
    }
    return true;
  };
  Conf.PostOptModuleTransform = [&](unsigned Task, Module &M) {
    if (!Task || Task >= States.size())
      return false;
    auto &S = States[Task];
    auto Finish = [&]() -> Error {
      for (GlobalValue &G : M.global_values()) {
        auto It = OriginalNames[Task - 1].find(G.getName());
        if (It == OriginalNames[Task - 1].end()) {
          if (TemporaryNames.count(G.getName()))
            return fail("imported an unresolved Go definition: " + G.getName());
          continue;
        }
        if (M.getNamedValue(It->second))
          return fail("conflicting restored Go symbol: " + It->second);
        G.setName(It->second);
        if (auto *GO = dyn_cast<GlobalObject>(&G))
          GO->setMetadata("guid", nullptr);
      }
      for (StringRef Name : PackageMetadataNames) {
        if (auto *MD = M.getNamedMetadata(Name))
          M.eraseNamedMetadata(MD);
        for (auto *MD : S.Metadata[Name])
          M.getOrInsertNamedMetadata(Name)->addOperand(MD);
      }
      if (Error E = finishGoObjModule(M, S.Owner, S.Slots))
        return E;
      if (Error E = Save(Task, M, ""))
        return E;
      if (AddStream) {
        if (Error E = goallc::prepareCodeGenModule(M, CodeGenOptLevel::Default))
          return E;
        if (verifyModule(M, &errs()))
          return fail("invalid ThinLTO pre-codegen module");
      }
      return Error::success();
    };
    if (Error E = Finish()) {
      S.Error = toString(std::move(E));
      return false;
    }
    S.Finished = true;
    return bool(AddStream);
  };
  lto::LTO Driver(std::move(Conf),
                  lto::createInProcessThinBackend(hardware_concurrency(Jobs)));
  StringSet<> Prevailing;
  for (auto &File : Files) {
    std::vector<lto::SymbolResolution> Resolutions;
    for (const auto &Sym : File->symbols()) {
      lto::SymbolResolution R;
      R.Prevailing = !Sym.isUndefined();
      if (R.Prevailing && !Prevailing.insert(Sym.getName()).second)
        return fail("unresolved duplicate LTO symbol: " + Sym.getName());
      R.VisibleToRegularObj = true;
      // Native objects and Go-specific resolution still participate later.
      // Do not assert finality or whole-program visibility here.
      Resolutions.push_back(R);
    }
    if (Error E = Driver.add(std::move(File), Resolutions))
      return E;
  }
  if (Driver.getMaxTasks() != States.size())
    return fail("unexpected LTO task count");
  auto Stream =
      [&](unsigned Task,
          const Twine &Name) -> Expected<std::unique_ptr<CachedFileStream>> {
    if (!Task || Task >= States.size())
      return fail("unexpected LTO output task");
    return AddStream(Task - 1, Name);
  };
  if (Error E = Driver.run(Stream))
    return E;
  for (unsigned Task = 1; Task < States.size(); ++Task) {
    if (!States[Task].Error.empty())
      return fail(States[Task].Error);
    if (!States[Task].Finished)
      return fail("LTO omitted a package output");
  }
  if (!Diagnostics.empty())
    return fail(Diagnostics);
  return Error::success();
}

extern "C" LLVMErrorRef LLVMGoALLCThinLTO(LLVMMemoryBufferRef *Inputs,
                                          size_t Count, const char *Diagnostics,
                                          LLVMBool PIC, unsigned Jobs,
                                          LLVMMemoryBufferRef *Outputs) {
  if (!Count || !Jobs)
    return LLVMCreateStringError(
        "ThinLTO requires inputs and a positive job count");
  SmallVector<MemoryBufferRef> Buffers;
  for (size_t I = 0; I != Count; ++I)
    Buffers.push_back(unwrap(Inputs[I])->getMemBufferRef());
  std::vector<SmallVector<char, 0>> Objects(Count);
  auto AddStream =
      [&](unsigned I,
          const Twine &) -> Expected<std::unique_ptr<CachedFileStream>> {
    return std::make_unique<CachedFileStream>(
        std::make_unique<raw_svector_ostream>(Objects[I]));
  };
  if (Error E = goallc::runThinLTO(Buffers, Diagnostics, Jobs, PIC, AddStream))
    return wrap(std::move(E));
  for (size_t I = 0; I != Count; ++I) {
    auto &Object = Objects[I];
    if (Diagnostics[0]) {
      std::error_code EC;
      ToolOutputFile F((Twine(Diagnostics) + "/" + Twine(I) + ".o").str(), EC,
                       sys::fs::OF_None);
      if (EC)
        return wrap(fail(EC.message()));
      F.os().write(Object.data(), Object.size());
      F.keep();
    }
    Outputs[I] = wrap(
        MemoryBuffer::getMemBufferCopy(StringRef(Object.data(), Object.size()),
                                       Buffers[I].getBufferIdentifier())
            .release());
  }
  return LLVMErrorSuccess;
}

extern "C" LLVMErrorRef
LLVMGoALLCWriteThinBitcode(LLVMModuleRef Module, LLVMMemoryBufferRef *Output) {
  SmallVector<char, 0> Buffer;
  raw_svector_ostream OS(Buffer);
  if (Error E = goallc::writeThinBitcode(*unwrap(Module), OS))
    return wrap(std::move(E));
  *Output = wrap(
      MemoryBuffer::getMemBufferCopy(StringRef(Buffer.data(), Buffer.size()),
                                     unwrap(Module)->getModuleIdentifier())
          .release());
  return LLVMErrorSuccess;
}
