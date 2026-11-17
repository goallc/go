// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

#include "GoALLCPreCodeGen.h"
#include "GoALLCThinLTOC.h"
#include "llvm/ADT/DenseSet.h"
#include "llvm/ADT/StringExtras.h"
#include "llvm/Analysis/ModuleSummaryAnalysis.h"
#include "llvm/Analysis/ProfileSummaryInfo.h"
#include "llvm/BinaryFormat/GoObj.h"
#include "llvm/BinaryFormat/GoObjIdentity.h"
#include "llvm/Bitcode/BitcodeReader.h"
#include "llvm/Bitcode/BitcodeWriter.h"
#include "llvm/IR/Constants.h"
#include "llvm/IR/DebugInfo.h"
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
static bool exportableReference(ValueInfo VI, const StringSet<> &Ambiguous) {
  const GlobalValue *G = VI.getValue();
  if (!G || Ambiguous.count(G->getName()))
    return false;
  StringRef Name = G->getName();
  Name.consume_back(GoObj::ABI0SymbolSuffix);
  if (GoObj::getSymbolSuffix(Name) == GoObj::LinknameSymbolSuffix)
    return false;
  if (G->isDeclaration() || Name.starts_with("llvm."))
    return true;
  const auto *GO = dyn_cast<GlobalObject>(G);
  return GO && !hashed(*GO) &&
         (GO->getMetadata("goobj.symbol.index") || !G->hasLocalLinkage());
}

// Conservative import policy for the current GoObj representation, not a
// ThinLTO requirement. Relax individual restrictions only when Go symbol
// ownership and metadata-only reachability survive the importer.
static bool canImportGoFunction(const Function &F, const FunctionSummary &Summary,
                                const StringSet<> &Ambiguous) {
  if (Ambiguous.count(F.getName()) || hashed(F) || F.isInterposable() ||
      F.getMetadata("goobj.keep") || F.getMetadata("goobj.marker_relocs") ||
      F.getMetadata("goobj.weak_calls"))
    return false;
  for (const BasicBlock &BB : F)
    for (const Instruction &I : BB) {
      // Relocation targets in metadata are not ThinLTO reference edges. Keep
      // these bodies in their owning package until they have an importable
      // representation; an imported null target would lose Go reachability.
      if (I.getMetadata("goobj.marker_reloc"))
        return false;
    }
  // Reuse LLVM's reference and call edges, including nested constant operands.
  auto Exportable = [&](ValueInfo VI) {
    return exportableReference(VI, Ambiguous);
  };
  return llvm::all_of(Summary.refs(), Exportable) &&
         llvm::all_of(Summary.calls(), [&](const auto &Edge) {
           return Exportable(Edge.first);
         });
}

static Error writeThinBitcode(Module &M, raw_ostream &OS) {
  auto Prefix = packagePrefix(M);
  if (!Prefix)
    return Prefix.takeError();
  LLVMContext &C = M.getContext();
  C.getMDKindID("goobj.llvm.identity");
  if (auto *Version = M.getModuleFlag("goobj.llvm.identity")) {
    auto *V = mdconst::dyn_extract<ConstantInt>(Version);
    if (!V || !V->equalsInt(1))
      return fail("unsupported GoObj LLVM identity version");
  } else {
    M.addModuleFlag(Module::Error, "goobj.llvm.identity", 1);
  }
  // Existing backends reject this schema rather than emitting encoded names.
  auto *ABI = dyn_cast<MDNode>(M.getModuleFlag("goobj.abi"));
  if (!ABI || ABI->getNumOperands() != 8)
    return fail("invalid GoObj ABI for stable identity");
  auto *Schema = dyn_cast<MDString>(ABI->getOperand(0));
  if (!Schema || (Schema->getString() != "goallc.goobj" &&
                  Schema->getString() != "goallc.goobj.llvm-identity.v1"))
    return fail("unsupported GoObj ABI for stable identity");
  SmallVector<Metadata *> ABIFields(ABI->op_begin(), ABI->op_end());
  ABIFields[0] = MDString::get(C, "goallc.goobj.llvm-identity.v1");
  M.setModuleFlag(Module::Error, "goobj.abi", MDNode::get(C, ABIFields));
  DenseSet<uint32_t> Indices;
  SmallVector<GlobalValue *> Retained;
  StringSet<> NativeOnly;
  StringMap<std::string> IRNames;
  SmallVector<std::pair<GlobalObject *, std::string>> NewNames;
  for (GlobalObject &G : M.global_objects()) {
    auto *F = dyn_cast<Function>(&G);
    auto *Index = G.getMetadata("goobj.symbol.index");
    StringRef Owner;
    uint64_t Slot = 0, Flags2 = 0;
    if (Index) {
      auto *I = Index->getNumOperands()
                    ? mdconst::dyn_extract<ConstantInt>(Index->getOperand(0))
                    : nullptr;
      if (!I || !I->getType()->isIntegerTy(32) || G.isDeclaration() ||
          !Indices.insert(I->getZExtValue()).second)
        return fail("invalid ThinLTO package index: " + G.getName());
      Owner = *Prefix;
      Slot = I->getZExtValue();
      if (auto *Flags = G.getMetadata("goobj.symbol.flags"))
        Flags2 = mdconst::extract<ConstantInt>(Flags->getOperand(1))
                     ->getZExtValue() &
                 GoObj::SymFlagUsedInIface;
    } else if (auto *Ref = G.getMetadata("goobj.import")) {
      auto *Package = Ref->getNumOperands() == 3
                          ? dyn_cast<MDString>(Ref->getOperand(0))
                          : nullptr;
      auto *I = Package ? mdconst::dyn_extract<ConstantInt>(Ref->getOperand(1))
                        : nullptr;
      auto *Flags = Package
                        ? mdconst::dyn_extract<ConstantInt>(Ref->getOperand(2))
                        : nullptr;
      if (!Package || Package->getString().empty() || !I ||
          I->getValue().ugt(UINT32_MAX) || !Flags ||
          Flags->getValue().ugt(UINT8_MAX))
        return fail("invalid ThinLTO function reference: " + G.getName());
      Owner = Package->getString();
      Slot = I->getZExtValue();
      Flags2 = Flags->getZExtValue();
    }
    if (Owner.empty() && F && F->hasFnAttribute("goobj.import.package")) {
      Owner = F->getFnAttribute("goobj.import.package").getValueAsString();
      if (F->getFnAttribute("goobj.import.index")
              .getValueAsString()
              .getAsInteger(10, Slot) ||
          F->getFnAttribute("goobj.import.flags2")
              .getValueAsString()
              .getAsInteger(10, Flags2))
        return fail("invalid existing Go identity");
    }
    if (!Owner.empty()) {
      if (F) {
        // Function::deleteBody drops metadata but preserves attributes.
        // Encode indexed definitions directly, without a metadata round trip.
        F->addFnAttr("goobj.import.package", Owner);
        F->addFnAttr("goobj.import.index", utostr(Slot));
        F->addFnAttr("goobj.import.flags2", utostr(Flags2));
        F->setMetadata("goobj.import", nullptr);
      } else {
        // Global metadata survives initializer removal during importing.
        Metadata *Ref[] = {MDString::get(C, Owner),
                           ConstantAsMetadata::get(
                               ConstantInt::get(Type::getInt32Ty(C), Slot)),
                           ConstantAsMetadata::get(
                               ConstantInt::get(Type::getInt32Ty(C), Flags2))};
        G.setMetadata("goobj.import", MDNode::get(C, Ref));
      }
    }
    if (!G.isDeclaration() && !G.getName().starts_with("llvm.") &&
        (Index || (!G.hasLocalLinkage() && !hashed(G))))
      Retained.push_back(&G);
    // Establish the LLVM identity once, before the initial summary.
    // Hex encoding keeps package bytes out of the reserved suffix syntax.
    bool Marked = F ? F->hasFnAttribute("goobj.llvm.identity")
                    : G.getMetadata("goobj.llvm.identity") != nullptr;
    bool Escape = !Marked && G.getName().starts_with("__goid$");
    if (Marked && Owner.empty() && !G.isDeclaration())
      NativeOnly.insert(G.getName());
    if (!Owner.empty() || Escape ||
        (!G.isDeclaration() && !G.hasLocalLinkage() &&
         !G.getName().starts_with("llvm."))) {
      std::string NativeName = G.getName().str();
      if (Marked) {
        auto *MD = G.getMetadata("goobj.llvm.identity");
        auto *Version =
            MD && MD->getNumOperands() == 1
                ? mdconst::dyn_extract<ConstantInt>(MD->getOperand(0))
                : nullptr;
        if (F ? F->getFnAttribute("goobj.llvm.identity").getValueAsString() !=
                    "1"
              : !Version || !Version->equalsInt(1))
          return fail("unsupported GoObj symbol identity version");
        StringRef Decoded = GoObj::decodeLLVMIdentity(NativeName);
        if (Decoded.empty())
          return fail("invalid stable Go identity: " + G.getName());
        NativeName = Decoded.str();
      }
      std::string IRName = GoObj::encodeLLVMIdentity(
          NativeName, Owner.empty() ? *Prefix : Owner,
          Owner.empty() ? std::nullopt : std::optional<uint32_t>(Slot));
      if (Escape && Owner.empty() && (G.isDeclaration() || G.hasLocalLinkage()))
        IRName = "__goid$v1$6E6174697665$r$" + NativeName;
      IRNames[NativeName] = IRName;
      if (G.getName() != IRName) {
        NewNames.emplace_back(&G, IRName);
        G.setMetadata("guid", nullptr);
      }
      if (F)
        F->addFnAttr("goobj.llvm.identity", "1");
      else
        G.setMetadata("goobj.llvm.identity",
                      MDNode::get(C, ConstantAsMetadata::get(ConstantInt::get(
                                         Type::getInt32Ty(C), 1))));
      if (Owner.empty() && !G.isDeclaration())
        NativeOnly.insert(IRName);
    }
  }
  // Release original names together: a valid native name may equal another
  // definition's encoded identity. References follow their GlobalValues.
  for (auto &[G, Name] : NewNames)
    G->setName("");
  for (auto &[G, Name] : NewNames) {
    if (M.getNamedValue(Name))
      return fail("conflicting stable Go identity: " + Name);
    G->setName(Name);
  }
  // Debug identities are scalar annotations, not GlobalValue references.
  DebugInfoFinder Debug;
  Debug.processModule(M);
  for (DISubprogram *SP : Debug.subprograms()) {
    auto *Annotations = SP->getAnnotations().get();
    if (!Annotations)
      continue;
    StringRef Name, Owner;
    bool Marked = false;
    const ConstantInt *Slot = nullptr;
    for (const MDOperand &Operand : Annotations->operands()) {
      auto *A = cast<MDNode>(Operand);
      if (A->getNumOperands() != 2)
        continue;
      auto *Key = dyn_cast<MDString>(A->getOperand(0));
      if (!Key)
        continue;
      if (Key->getString() == "goobj.llvm.identity")
        Marked = true;
      if (Key->getString() == "goobj.symbol")
        Name = cast<MDString>(A->getOperand(1))->getString();
      if (Key->getString() == "goobj.package")
        Owner = cast<MDString>(A->getOperand(1))->getString();
      if (Key->getString() == "goobj.index")
        Slot = mdconst::dyn_extract<ConstantInt>(A->getOperand(1));
    }
    if (Marked) {
      if (GoObj::decodeLLVMIdentity(Name).empty())
        return fail("invalid stable Go debug identity");
      continue;
    }
    std::string IRName = IRNames.lookup(Name);
    if (IRName.empty() && !Owner.empty() && Slot)
      IRName = GoObj::encodeLLVMIdentity(Name, Owner, Slot->getZExtValue());
    if (IRName.empty())
      continue;
    SmallVector<Metadata *> Updated;
    for (const MDOperand &Operand : Annotations->operands()) {
      auto *A = cast<MDNode>(Operand);
      auto *Key = A->getNumOperands() == 2
                      ? dyn_cast<MDString>(A->getOperand(0))
                      : nullptr;
      Updated.push_back(Key && Key->getString() == "goobj.symbol"
                            ? MDNode::get(C, {Key, MDString::get(C, IRName)})
                            : A);
    }
    Updated.push_back(MDNode::get(C, {MDString::get(C, "goobj.llvm.identity"),
                                      ConstantAsMetadata::get(ConstantInt::get(
                                          Type::getInt32Ty(C), 1))}));
    SP->replaceOperandWith(11, MDTuple::get(C, Updated));
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
    for (auto &Summary : VI.getSummaryList()) {
      auto *FS = dyn_cast<FunctionSummary>(Summary.get());
      if (!FS || !canImportGoFunction(cast<Function>(G), *FS, NativeOnly))
        Summary->setNotEligibleToImport();
    }
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
            MetadataNames[Entry.first] != "goobj.import" &&
            MetadataNames[Entry.first] != "goobj.llvm.identity")
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

static Error runThinLTO(ArrayRef<MemoryBufferRef> Inputs, StringRef OutDir,
                        unsigned Jobs, bool PIC,
                        MutableArrayRef<SmallVector<char, 0>> Objects) {
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
  // Stable identities have already been established at first serialization.
  std::vector<std::unique_ptr<lto::InputFile>> Files;
  StringSet<> Names;
  for (MemoryBufferRef Input : Inputs) {
    StringRef Name = Input.getBufferIdentifier();
    if (Name.empty() || !Names.insert(Name).second)
      return fail("ThinLTO requires unique nonempty module identifiers");
    auto File = lto::InputFile::create(Input);
    if (!File)
      return File.takeError();
    Files.push_back(std::move(*File));
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
      BC.os() << Inputs[Task - 1].getBuffer();
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
    auto *Version = mdconst::dyn_extract_or_null<ConstantInt>(
        M.getModuleFlag("goobj.llvm.identity"));
    if (!Version || !Version->equalsInt(1)) {
      S.Error = "ThinLTO requires stable Go identity version 1";
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
      if (Error E = goallc::prepareCodeGenModule(M, CodeGenOptLevel::Default))
        return E;
      if (verifyModule(M, &errs()))
        return fail("invalid ThinLTO pre-codegen module");
      return Error::success();
    };
    if (Error E = Finish()) {
      S.Error = toString(std::move(E));
      return false;
    }
    S.Finished = true;
    return true;
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
          const Twine &) -> Expected<std::unique_ptr<CachedFileStream>> {
    if (!Task || Task >= States.size())
      return fail("unexpected LTO output task");
    return std::make_unique<CachedFileStream>(
        std::make_unique<raw_svector_ostream>(Objects[Task - 1]));
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
  if (Error E = runThinLTO(Buffers, Diagnostics, Jobs, PIC, Objects))
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
  if (Error E = writeThinBitcode(*unwrap(Module), OS))
    return wrap(std::move(E));
  *Output = wrap(
      MemoryBuffer::getMemBufferCopy(StringRef(Buffer.data(), Buffer.size()),
                                     unwrap(Module)->getModuleIdentifier())
          .release());
  return LLVMErrorSuccess;
}
