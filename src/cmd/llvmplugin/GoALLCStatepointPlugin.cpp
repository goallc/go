// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCPreCodeGen.h"
#include "GoALLCCPUFeatures.h"
#include "GoALLCStatepoints.h"
#include "llvm-c/Core.h"
#include "llvm-c/Error.h"
#include "llvm-c/TargetMachine.h"
#include "llvm/ADT/SmallVector.h"
#include "llvm/Analysis/AssumptionCache.h"
#include "llvm/CodeGen/StackProtector.h"
#include "llvm/CodeGen/TargetPassConfig.h"
#include "llvm/Config/llvm-config.h"
#include "llvm/IR/LLVMContext.h"
#include "llvm/IR/Module.h"
#include "llvm/Pass.h"
#include "llvm/Passes/PassBuilder.h"
#include "llvm/Plugins/PassPlugin.h"
#include "llvm/Support/CBindingWrapping.h"
#include "llvm/Support/CommandLine.h"
#include "llvm/Support/Error.h"
#include "llvm/Support/ErrorHandling.h"
#include "llvm/Support/raw_ostream.h"
#include "llvm/Target/RegisterTargetPassConfigCallback.h"
#include "llvm/Target/TargetMachine.h"

using namespace llvm;

Pass *createGoALLCInlineAnchorPass();
void linkGoALLCStackMapPrinter();

namespace {

cl::opt<bool> ReportInvocation(
    "goallc-pass-plugin-report", cl::Hidden,
    cl::desc("Report invocation of the GoALLC pre-codegen plugin"),
    cl::init(false));

cl::opt<bool>
    EmitIR("goallc-pass-plugin-emit-ir", cl::Hidden,
           cl::desc("Emit IR after the GoALLC pre-codegen pipeline and stop"),
           cl::init(false));

void registerGoALLCTargetPasses();

void registerGoALLCPassBuilderCallbacks(PassBuilder &PB) {
  PB.registerPipelineParsingCallback(
      [](StringRef Name, ModulePassManager &MPM,
         ArrayRef<PassBuilder::PipelineElement>) {
        if (Name != "goallc-cpu-features")
          return false;
        MPM.addPass(goallc::CPUFeaturesPass());
        return true;
      });
}

bool runPreCodeGenCallback(Module &M, TargetMachine &TM, CodeGenFileType,
                           raw_pwrite_stream &) {
  // A dynamically loaded plugin runs its static constructors immediately
  // before this callback. A plugin linked into compile, however, may run them
  // before LLVM has initialized its callback registry. Register lazily from
  // the common callback so both linkage modes observe the same lifetime.
  registerGoALLCTargetPasses();

  if (Error Err = goallc::runEarlyIRPipeline(M)) {
    M.getContext().emitError(toString(std::move(Err)));
    return true;
  }

  // Keep the early callback only as an IR-emission test facility. Production
  // compilation performs only module-wide preparation here, then rewrites
  // each function from GoALLCPreISelPass after standard codegen IR preparation.
  Error Err = EmitIR ? goallc::runPreCodeGenPipeline(M, TM)
                     : goallc::prepareStatepointModule(M);
  if (Err) {
    M.getContext().emitError(toString(std::move(Err)));
    return true;
  }

  if (!EmitIR)
    return false;

  if (ReportInvocation)
    errs() << "GoALLCStatepoints: ran pre-codegen pipeline for "
           << M.getModuleIdentifier() << '\n';
  if (EmitIR) {
    M.print(errs(), nullptr);
    return true;
  }
  return false;
}

class GoALLCPreISelPass final : public FunctionPass {
public:
  static char ID;

  GoALLCPreISelPass() : FunctionPass(ID) {}
  explicit GoALLCPreISelPass(TargetMachine &TM) : FunctionPass(ID), TM(&TM) {}

  bool runOnFunction(Function &F) override {
    assert(TM && "GoALLC pre-isel pass requires a target machine");
    if (Error Err = goallc::rewriteStatepoints(F, *TM)) {
      std::string Message = toString(std::move(Err));
      F.getContext().emitError(Message);
      // Validation can fail after canonicalization has already changed local
      // IR, so conservatively invalidate analyses even though code generation
      // will stop on the emitted diagnostic.
      return true;
    }
    if (ReportInvocation)
      errs() << "GoALLCStatepoints: ran late pre-isel pipeline for "
             << F.getName() << '\n';
    return true;
  }

  void getAnalysisUsage(AnalysisUsage &AU) const override {
    // rewriteStatepoints changes both instructions and the CFG. Let the legacy
    // pass manager invalidate and rebuild CFG-dependent analyses before
    // instruction selection instead of carrying pre-rewrite DT/AA state across
    // this pass. The rewrite does not add or remove assumptions, and its
    // temporary allocas are promoted before returning, so the already-computed
    // stack-protector layout remains valid.
    AU.addPreserved<AssumptionCacheTracker>();
    AU.addPreserved<StackProtector>();
  }

  StringRef getPassName() const override { return "GoALLC late statepoints"; }

private:
  TargetMachine *TM = nullptr;
};

char GoALLCPreISelPass::ID = 0;
static RegisterPass<GoALLCPreISelPass>
    RegisterGoALLCPreISelPass("goallc-late-statepoints",
                              "GoALLC late statepoints", false, false);

void registerGoALLCTargetPasses() {
  static RegisterTargetPassConfigCallback Registration(
      [](TargetMachine &TM, PassManagerBase &, TargetPassConfig *TPC) {
        if (!TPC)
          return;

        TPC->addPreISelPass(
            [TMPtr = &TM]() { return new GoALLCPreISelPass(*TMPtr); });
        if (TM.getTargetTriple().isOSBinFormatGoObj())
          TPC->addPreBranchRelaxationPass(
              []() { return createGoALLCInlineAnchorPass(); });
      });
  (void)Registration;
}

} // namespace

extern "C" LLVMErrorRef LLVMGoALLCRunEarlyIR(LLVMModuleRef ModuleRef) {
  Module &M = *unwrap(ModuleRef);
  if (Error Err = goallc::runEarlyIRPipeline(M)) {
    std::string Message = toString(std::move(Err));
    return LLVMCreateStringError(Message.c_str());
  }
  return LLVMErrorSuccess;
}

extern "C" LLVMErrorRef
LLVMGoALLCRunPreCodeGen(LLVMModuleRef ModuleRef,
                        LLVMTargetMachineRef TargetMachineRef,
                        LLVMCodeGenFileType FileType) {
  linkGoALLCStackMapPrinter();
  SmallVector<char, 0> CallbackOutput;
  raw_svector_ostream OS(CallbackOutput);
  Module &M = *unwrap(ModuleRef);
  TargetMachine &TM = *reinterpret_cast<TargetMachine *>(TargetMachineRef);
  if (runPreCodeGenCallback(M, TM, static_cast<CodeGenFileType>(FileType), OS))
    return LLVMCreateStringError(
        "linked GoALLC plugin stopped the in-process code-generation pipeline");
  return LLVMErrorSuccess;
}

extern "C" LLVM_ATTRIBUTE_WEAK ::llvm::PassPluginLibraryInfo
llvmGetPassPluginInfo() {
  return {LLVM_PLUGIN_API_VERSION, "GoALLCStatepoints", LLVM_VERSION_STRING,
          registerGoALLCPassBuilderCallbacks, runPreCodeGenCallback};
}
