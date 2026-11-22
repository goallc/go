// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCPGO.h"
#include "llvm/ADT/SmallPtrSet.h"
#include "llvm/IR/IRBuilder.h"
#include "llvm/IR/Module.h"
#include "llvm/ProfileData/InstrProf.h"
#include "llvm/Support/Error.h"
#include "llvm/Transforms/Utils/ModuleUtils.h"

using namespace llvm;

// The native instrprof pass owns counter placement, allocation and updates.
// Translate only its registration data to the Go linker's package table.
static Error adaptProfileData(Module &M) {
  IRBuilder<> B(M.getContext());
  auto *Word = M.getDataLayout().getIntPtrType(M.getContext());
  GlobalVariable *Names = M.getNamedGlobal(getInstrProfNamesVarName());
  InstrProfSymtab Symtab;
  if (Names) {
    if (!Names->hasInitializer())
      return createStringError("missing LLVM profile names");
    auto *Bytes = dyn_cast_or_null<ConstantDataArray>(Names->getInitializer());
    if (!Bytes || !Bytes->isString())
      return createStringError("invalid LLVM profile names");
    if (Error Err = Symtab.create(Bytes->getAsString()))
      return Err;
  }

  SmallVector<Constant *> Records;
  SmallVector<GlobalVariable *> Descriptors;
  auto DataSection =
      getInstrProfSectionName(IPSK_data, M.getTargetTriple().getObjectFormat());
  for (GlobalVariable &GV : M.globals()) {
    if (GV.getSection() != DataSection)
      continue;
    // LLVM 23 InstrProfData.inc: NameRef, FuncHash and CounterPtr.
    auto *Data = GV.hasInitializer()
                     ? dyn_cast<ConstantStruct>(GV.getInitializer())
                     : nullptr;
    if (!Data || Data->getNumOperands() < 3)
      return createStringError("invalid LLVM profile descriptor");
    auto *NameRef = dyn_cast<ConstantInt>(Data->getOperand(0));
    auto *Hash = dyn_cast<ConstantInt>(Data->getOperand(1));
    auto *Delta = dyn_cast<ConstantExpr>(Data->getOperand(2));
    if (!NameRef || !Hash || !Delta || Delta->getOpcode() != Instruction::Sub)
      return createStringError("invalid LLVM profile counter offset");
    auto pointer = [](Constant *C) -> Value * {
      auto *Cast = dyn_cast<ConstantExpr>(C);
      return Cast && Cast->getOpcode() == Instruction::PtrToInt
                 ? Cast->getOperand(0)
                 : nullptr;
    };
    if (pointer(Delta->getOperand(1)) != &GV)
      return createStringError("invalid LLVM profile descriptor base");
    auto *Counters =
        dyn_cast_or_null<GlobalVariable>(pointer(Delta->getOperand(0)));
    auto *Ty =
        Counters ? dyn_cast<ArrayType>(Counters->getValueType()) : nullptr;
    StringRef Name = Symtab.getFuncOrVarName(NameRef->getZExtValue());
    if (!Ty || !Ty->getElementType()->isIntegerTy(64) || Name.empty())
      return createStringError("invalid LLVM profile counter array or name");
    // Anonymous data cannot carry Go ABI suffixes. The profile string keeps
    // the full LLVM name, including any local linkage disambiguation.
    Counters->setName("go:llvmpgocount");
    Counters->setComdat(nullptr);
    Counters->setLinkage(GlobalValue::InternalLinkage);
    Counters->setSection(".noptrbss");
    auto *Text = ConstantDataArray::getString(M.getContext(), Name, false);
    auto *NameVar =
        new GlobalVariable(M, Text->getType(), true,
                           GlobalValue::PrivateLinkage, Text, "go:llvmpgoname");
    auto *Len = ConstantInt::get(Word, Ty->getNumElements());
    Records.push_back(
        ConstantStruct::getAnon({NameVar, ConstantInt::get(Word, Name.size()),
                                 Hash, Counters, Len, Len}));
    Descriptors.push_back(&GV);
  }

  SmallVector<GlobalValue *> Runtime;
  for (StringRef Name :
       {getInstrProfInitFuncName(), getInstrProfRegFuncsName(),
        getInstrProfRuntimeHookVarUseFuncName(), getInstrProfRegFuncName(),
        getInstrProfNamesRegFuncName(), getInstrProfRuntimeHookVarName(),
        StringRef(INSTR_PROF_QUOTE(INSTR_PROF_RAW_VERSION_VAR))})
    if (auto *GV = M.getNamedValue(Name))
      Runtime.push_back(GV);
  if (Names)
    Runtime.push_back(Names);
  if (Function *Init = M.getFunction(getInstrProfInitFuncName()))
    transformGlobalCtors(M, [&](Constant *Ctor) -> Constant * {
      return Ctor->getAggregateElement(1u)->stripPointerCasts() == Init
                 ? nullptr
                 : Ctor;
    });
  Runtime.append(Descriptors.begin(), Descriptors.end());
  SmallPtrSet<GlobalValue *, 16> Removed(Runtime.begin(), Runtime.end());
  removeFromUsedLists(M, [&](Constant *C) {
    return Removed.contains(dyn_cast<GlobalValue>(C->stripPointerCasts()));
  });
  // Break self-relative descriptor references and calls between the profile
  // runtime helpers before checking for unexpected surviving users.
  for (auto *GV : Runtime) {
    if (auto *F = dyn_cast<Function>(GV))
      F->deleteBody();
    else
      cast<GlobalVariable>(GV)->setInitializer(nullptr);
  }
  for (auto *GV : Runtime) {
    GV->removeDeadConstantUsers();
    if (!GV->use_empty())
      return createStringError("unexpected use of LLVM profiling symbol %s",
                               GV->getName().str().c_str());
    GV->eraseFromParent();
  }
  // LL/SC expansion can split the entry block at an atomic increment. Keep
  // fixed frame slots above it even in unoptimized training builds.
  for (Function &F : M) {
    if (F.isDeclaration())
      continue;
    auto &Entry = F.getEntryBlock();
    auto IP = Entry.getFirstInsertionPt();
    for (Instruction &I : make_early_inc_range(Entry))
      if (auto *A = dyn_cast<AllocaInst>(&I);
          A && A->isStaticAlloca() && &I != &*IP)
        A->moveBefore(Entry, IP);
  }
  if (!Records.empty()) {
    auto *Ty = ArrayType::get(Records.front()->getType(), Records.size());
    auto *Data =
        new GlobalVariable(M, Ty, false, GlobalValue::InternalLinkage,
                           ConstantArray::get(Ty, Records),
                           "go:llvmpgorecords." + M.getSourceFileName());
    // These pointers address static data and need loader fixups in a PIE.
    // A FIPS package classifies local read-only constants as hashed bytes in
    // the text segment, where the platform loader cannot apply those fixups.
    Data->setSection(".noptrdata");
    auto *Len = ConstantInt::get(Word, Records.size());
    auto *Slice = ConstantStruct::getAnon({Data, Len, Len});
    auto *Table = new GlobalVariable(M, Slice->getType(), true,
                                     GlobalValue::ExternalLinkage, Slice,
                                     "go:llvmpgopkg." + M.getSourceFileName());
    Table->setSection(".rodata");
    appendToCompilerUsed(M, {Table});
  }
  return Error::success();
}

PreservedAnalyses goallc::AdaptPGODataPass::run(Module &M,
                                                ModuleAnalysisManager &) {
  if (Error Err = adaptProfileData(M))
    M.getContext().emitError(toString(std::move(Err)));
  return PreservedAnalyses::none();
}

extern "C" LLVM_EXTERNAL_VISIBILITY LLVMErrorRef
LLVMGoALLCAdaptPGOData(LLVMModuleRef Mod) {
  return wrap(adaptProfileData(*unwrap(Mod)));
}
