// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCPGO.h"
#include "llvm/IR/DiagnosticHandler.h"
#include "llvm/IR/IRBuilder.h"
#include "llvm/IR/IntrinsicInst.h"
#include "llvm/Passes/PassBuilder.h"
#include "llvm/ProfileData/InstrProf.h"
#include "llvm/Transforms/Instrumentation/PGOInstrumentation.h"
#include "llvm/Transforms/Utils/ModuleUtils.h"

using namespace llvm;

// GoObj programs have neither compiler-rt's profile sections nor its C exit
// hooks. Keep LLVM's CFG hashes and counter placement, but publish the counter
// arrays to the Go linker and use atomic increments across goroutines.
static Error lowerGoCounters(Module &M) {
  IRBuilder<> B(M.getContext());
  auto *Word = M.getDataLayout().getIntPtrType(M.getContext());
  SmallVector<Constant *> Records;
  for (Function &F : M) {
    GlobalVariable *Counters = nullptr;
    for (BasicBlock &BB : F) {
      for (Instruction &I : make_early_inc_range(BB)) {
        auto *Prof = dyn_cast<InstrProfInstBase>(&I);
        if (!Prof)
          continue;
        auto *Inc = dyn_cast<InstrProfIncrementInst>(Prof);
        if (!Inc)
          return createStringError(
              "unsupported LLVM PGO intrinsic: %s",
              Prof->getCalledFunction()->getName().str().c_str());
        if (!Counters) {
          uint64_t N = Inc->getNumCounters()->getZExtValue();
          auto *Ty = ArrayType::get(B.getInt64Ty(), N);
          Counters = new GlobalVariable(
              M, Ty, false, GlobalValue::InternalLinkage,
              ConstantAggregateZero::get(Ty), "go:llvmpgocount");
          Counters->setAlignment(Align(8));
          Counters->setSection(".noptrbss");
          auto *Name = Inc->getName();
          // An ABI0 suffix describes code in GoObj. Keep it in the profile
          // string, but never put it on a data symbol.
          Name->setName("go:llvmpgoname");
          // Matches record in runtime/llvmprofile (string, hash, []uint64).
          auto *Len = ConstantInt::get(Word, N);
          Records.push_back(ConstantStruct::getAnon(
              {Name,
               ConstantInt::get(Word,
                                getPGOFuncNameVarInitializer(Name).size()),
               Inc->getHash(), Counters, Len, Len}));
        }
        B.SetInsertPoint(Inc);
        Value *Addr = B.CreateInBoundsGEP(Counters->getValueType(), Counters,
                                          {B.getInt32(0), Inc->getIndex()});
        B.CreateAtomicRMW(AtomicRMWInst::Add, Addr, Inc->getStep(), Align(8),
                          AtomicOrdering::Monotonic);
        Inc->eraseFromParent();
      }
    }
    if (Counters) {
      // On targets using LL/SC, AtomicExpand may split the entry block at a
      // counter. Keep fixed frame slots in the entry block after that split.
      auto IP = F.getEntryBlock().getFirstInsertionPt();
      for (Instruction &I : make_early_inc_range(F.getEntryBlock()))
        if (auto *A = dyn_cast<AllocaInst>(&I);
            A && A->isStaticAlloca() && &I != &*IP)
          A->moveBefore(F.getEntryBlock(), IP);
    }
  }
  M.getNamedGlobal(INSTR_PROF_QUOTE(INSTR_PROF_RAW_VERSION_VAR))
      ->eraseFromParent();
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

Error llvm::goallc::runPGO(Module &M) {
  NamedMDNode *Config = M.getNamedMetadata("goallc.pgo");
  if (!Config)
    return Error::success();
  if (Config->getNumOperands() != 1 ||
      Config->getOperand(0)->getNumOperands() != 1)
    return createStringError("invalid GoALLC PGO configuration");
  auto *Mode = dyn_cast<MDString>(Config->getOperand(0)->getOperand(0));
  if (!Mode)
    return createStringError("expected a GoALLC PGO mode or profile path");
  std::string Profile = Mode->getString().str();
  Config->eraseFromParent();

  LoopAnalysisManager LAM;
  FunctionAnalysisManager FAM;
  CGSCCAnalysisManager CGAM;
  ModuleAnalysisManager MAM;
  PassBuilder PB;
  PB.registerModuleAnalyses(MAM);
  PB.registerCGSCCAnalyses(CGAM);
  PB.registerFunctionAnalyses(FAM);
  PB.registerLoopAnalyses(LAM);
  PB.crossRegisterProxies(LAM, FAM, CGAM, MAM);

  if (Profile == "generate") {
    if (M.getNamedValue(INSTR_PROF_QUOTE(INSTR_PROF_RAW_VERSION_VAR)))
      return createStringError("LLVM profile version symbol already defined");
    PGOInstrumentationGen().run(M, MAM);
    return lowerGoCounters(M);
  }
  PGOInstrumentationUse(Profile).run(M, MAM);
  if (M.getContext().getDiagHandlerPtr()->HasErrors)
    return createStringError("could not apply LLVM PGO profile %s",
                             Profile.c_str());
  return Error::success();
}
