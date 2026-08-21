// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCPreCodeGen.h"
#include "GoALLCStatepoints.h"
#include "llvm/Analysis/AssumptionCache.h"
#include "llvm/CodeGen/StackProtector.h"
#include "llvm/CodeGen/TargetPassConfig.h"
#include "llvm/Config/llvm-config.h"
#include "llvm/IR/LLVMContext.h"
#include "llvm/IR/Module.h"
#include "llvm/Pass.h"
#include "llvm/Plugins/PassPlugin.h"
#include "llvm/Support/CommandLine.h"
#include "llvm/Support/raw_ostream.h"
#include "llvm/Target/RegisterTargetPassConfigCallback.h"
#include "llvm/Target/TargetMachine.h"

using namespace llvm;

Pass *createGoALLCInlineAnchorPass();

namespace {

cl::opt<bool> ReportInvocation(
    "goallc-pass-plugin-report", cl::Hidden,
    cl::desc("Report invocation of the GoALLC pre-codegen plugin"),
    cl::init(false));

cl::opt<bool>
    EmitIR("goallc-pass-plugin-emit-ir", cl::Hidden,
           cl::desc("Emit IR after the GoALLC pre-codegen pipeline and stop"),
           cl::init(false));

bool runPreCodeGenCallback(Module &M, TargetMachine &TM, CodeGenFileType,
                           raw_pwrite_stream &) {
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

RegisterTargetPassConfigCallback RegisterGoALLCTargetPasses(
    [](TargetMachine &TM, PassManagerBase &, TargetPassConfig *TPC) {
      if (!TPC)
        return;

      TPC->addPreISelPass(
          [TMPtr = &TM]() { return new GoALLCPreISelPass(*TMPtr); });
      if (TM.getTargetTriple().isOSBinFormatGoObj())
        TPC->addPreBranchRelaxationPass(
            []() { return createGoALLCInlineAnchorPass(); });
    });

} // namespace

extern "C" LLVM_ATTRIBUTE_WEAK ::llvm::PassPluginLibraryInfo
llvmGetPassPluginInfo() {
  return {LLVM_PLUGIN_API_VERSION, "GoALLCStatepoints", LLVM_VERSION_STRING,
          nullptr, runPreCodeGenCallback};
}
