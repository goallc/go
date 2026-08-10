// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCPreCodeGen.h"
#include "llvm/CodeGen/TargetPassConfig.h"
#include "llvm/Config/llvm-config.h"
#include "llvm/IR/LLVMContext.h"
#include "llvm/IR/Module.h"
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
  if (Error Err = goallc::runPreCodeGenPipeline(M, TM)) {
    M.getContext().emitError(toString(std::move(Err)));
    return true;
  }

  if (ReportInvocation)
    errs() << "GoALLCStatepoints: ran pre-codegen pipeline for "
           << M.getModuleIdentifier() << '\n';
  if (EmitIR) {
    M.print(errs(), nullptr);
    return true;
  }
  return false;
}

RegisterTargetPassConfigCallback RegisterGoALLCInlineAnchors(
    [](TargetMachine &TM, PassManagerBase &, TargetPassConfig *TPC) {
      if (TPC && TM.getTargetTriple().isOSBinFormatGoObj())
        TPC->addPreBranchRelaxationPass(
            []() { return createGoALLCInlineAnchorPass(); });
    });

} // namespace

extern "C" LLVM_ATTRIBUTE_WEAK ::llvm::PassPluginLibraryInfo
llvmGetPassPluginInfo() {
  return {LLVM_PLUGIN_API_VERSION, "GoALLCStatepoints", LLVM_VERSION_STRING,
          nullptr, runPreCodeGenCallback};
}
