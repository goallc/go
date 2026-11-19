// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCPGO.h"
#include "llvm-c/TargetMachine.h"
#include "llvm/IR/DiagnosticHandler.h"
#include "llvm/IR/IRBuilder.h"
#include "llvm/IR/IntrinsicInst.h"
#include "llvm/Passes/PassBuilder.h"
#include "llvm/Passes/StandardInstrumentations.h"
#include "llvm/ProfileData/InstrProf.h"
#include "llvm/Target/TargetMachine.h"
#include "llvm/Transforms/Instrumentation/InstrProfiling.h"
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
  auto *Version =
      M.getNamedGlobal(INSTR_PROF_QUOTE(INSTR_PROF_RAW_VERSION_VAR));
  if (!Version || !Version->use_empty())
    return createStringError(
        "expected an unused LLVM IR profile version symbol");
  Version->eraseFromParent();
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

PreservedAnalyses goallc::LowerPGOCountersPass::run(Module &M,
                                                    ModuleAnalysisManager &) {
  if (Error Err = lowerGoCounters(M))
    M.getContext().emitError(toString(std::move(Err)));
  return PreservedAnalyses::none();
}

namespace {
// LLVM 23 schedules compiler-rt lowering immediately after instrumentation
// and optional loop rotation. Keep the pipeline chosen by PassBuilder and
// substitute only the target-specific lowering at that position.
class GoPGOPipeline : public ModulePassManager {
public:
  Error configure(bool Generate, StringRef Profile) {
    unsigned GenSites = 0, LowerSites = 0, UseSites = 0;
    for (auto &P : Passes) {
      if (P->name() == PGOInstrumentationGen::name())
        ++GenSites;
      else if (P->name() == InstrProfilingLoweringPass::name()) {
        GoPGOPipeline Lower;
        Lower.addPass(goallc::LowerPGOCountersPass());
        P = std::move(Lower.Passes.front());
        ++LowerSites;
      } else if (P->name() == PGOInstrumentationUse::name()) {
        ++UseSites;
      }
    }
    if (GenSites != LowerSites || GenSites > 1 || UseSites > 1 ||
        (Generate ? UseSites : GenSites))
      return createStringError(
          "conflicting LLVM PGO passes in optimization pipeline");
    if (!GenSites && !UseSites) {
      // Custom pass lists (including 'none') have no LLVM-selected site.
      // Instrument their input so training and use see the same CFG.
      ModulePassManager Custom;
      if (Generate) {
        Custom.addPass(PGOInstrumentationGen());
        Custom.addPass(goallc::LowerPGOCountersPass());
      } else {
        Custom.addPass(PGOInstrumentationUse(Profile.str()));
      }
      ModulePassManager Tail(std::move(*this));
      Custom.addPass(std::move(Tail));
      addPass(std::move(Custom));
    }
    return Error::success();
  }
};
} // namespace

extern "C" LLVM_EXTERNAL_VISIBILITY LLVMErrorRef
LLVMGoALLCRunPGOPipeline(LLVMModuleRef Mod, LLVMTargetMachineRef Machine,
                         const char *Pipeline, const char *Mode) {
  Module &M = *unwrap(Mod);
  auto *TM = reinterpret_cast<TargetMachine *>(Machine);
  bool Generate = StringRef(Mode) == "generate";
  if (Generate && M.getNamedValue(INSTR_PROF_QUOTE(INSTR_PROF_RAW_VERSION_VAR)))
    return LLVMCreateStringError("LLVM profile version symbol already defined");
  PGOOptions Options(Generate ? "" : Mode, "", "", "",
                     Generate ? PGOOptions::IRInstr : PGOOptions::IRUse);
  TM->setPGOOption(Options);
  PassInstrumentationCallbacks PIC;
  PassBuilder PB(TM, PipelineTuningOptions(), Options, &PIC);
  LoopAnalysisManager LAM;
  FunctionAnalysisManager FAM;
  CGSCCAnalysisManager CGAM;
  ModuleAnalysisManager MAM;
  PB.registerModuleAnalyses(MAM);
  PB.registerCGSCCAnalyses(CGAM);
  PB.registerFunctionAnalyses(FAM);
  PB.registerLoopAnalyses(LAM);
  PB.crossRegisterProxies(LAM, FAM, CGAM, MAM);
  StandardInstrumentations SI(M.getContext(), false);
  SI.registerCallbacks(PIC, &MAM);

  GoPGOPipeline MPM;
  if (*Pipeline && StringRef(Pipeline) != "none")
    if (Error Err = PB.parsePassPipeline(MPM, Pipeline))
      return wrap(std::move(Err));
  if (Error Err = MPM.configure(Generate, Mode))
    return wrap(std::move(Err));
  MPM.run(M, MAM);
  if (M.getContext().getDiagHandlerPtr()->HasErrors)
    return LLVMCreateStringError("LLVM PGO pipeline reported an error");
  return LLVMErrorSuccess;
}
