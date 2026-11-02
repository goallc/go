//===- SupportBindings.cpp - Additional bindings for support --------------===//
//
// Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
// See https://llvm.org/LICENSE.txt for license information.
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception
//
//===----------------------------------------------------------------------===//
//
// This file defines additional C bindings for the support component.
//
//===----------------------------------------------------------------------===//

#include "SupportBindings.h"
#include "llvm/ADT/SmallVector.h"
#include "llvm/CodeGen/CommandFlags.h"
#include "llvm/IR/Module.h"
#include "llvm/IR/DiagnosticHandler.h"
#include "llvm/Passes/PassBuilder.h"
#include "llvm/Plugins/PassPlugin.h"
#include "llvm/Support/CBindingWrapping.h"
#include "llvm/Support/DynamicLibrary.h"
#include "llvm/Support/Error.h"
#include "llvm/Support/raw_ostream.h"
#include "llvm/Target/TargetMachine.h"
#include <stdlib.h>
#include <string.h>

using namespace llvm;

LLVMErrorRef LLVMRunPassPluginEarlyIR(LLVMModuleRef ModuleRef,
                                      const char *Filename) {
  Expected<PassPlugin> Plugin = PassPlugin::Load(Filename);
  if (!Plugin)
    return wrap(Plugin.takeError());

  LoopAnalysisManager LAM;
  FunctionAnalysisManager FAM;
  CGSCCAnalysisManager CGAM;
  ModuleAnalysisManager MAM;
  PassBuilder PB;
  Plugin->registerPassBuilderCallbacks(PB);
  PB.registerModuleAnalyses(MAM);
  PB.registerCGSCCAnalyses(CGAM);
  PB.registerFunctionAnalyses(FAM);
  PB.registerLoopAnalyses(LAM);
  PB.crossRegisterProxies(LAM, FAM, CGAM, MAM);

  ModulePassManager MPM;
  if (Error Err = PB.parsePassPipeline(MPM, "goallc-cpu-features"))
    return wrap(std::move(Err));
  Module &M = *unwrap(ModuleRef);
  bool HadErrors = M.getContext().getDiagHandlerPtr()->HasErrors;
  MPM.run(M, MAM);
  if (!HadErrors && M.getContext().getDiagHandlerPtr()->HasErrors)
    return LLVMCreateStringError("GoALLC early IR pass reported an error");
  return LLVMErrorSuccess;
}

void LLVMLoadLibraryPermanently2(const char *Filename, char **ErrMsg) {
  std::string ErrMsgStr;
  if (llvm::sys::DynamicLibrary::LoadLibraryPermanently(Filename, &ErrMsgStr)) {
    *ErrMsg = static_cast<char *>(malloc(ErrMsgStr.size() + 1));
    memcpy(static_cast<void *>(*ErrMsg),
           static_cast<const void *>(ErrMsgStr.c_str()), ErrMsgStr.size() + 1);
  }
}

LLVMBool LLVMGoALLCContextHasErrors(LLVMContextRef ContextRef) {
  return unwrap(ContextRef)->getDiagHandlerPtr()->HasErrors;
}

LLVMErrorRef LLVMConfigureGoObjFromModule(LLVMModuleRef ModuleRef) {
  return wrap(codegen::configureGoObjFromModule(*unwrap(ModuleRef)));
}

LLVMErrorRef LLVMRunPassPluginPreCodeGen(LLVMModuleRef ModuleRef,
                                         LLVMTargetMachineRef TargetMachineRef,
                                         LLVMCodeGenFileType FileType,
                                         const char *Filename) {
  Expected<PassPlugin> Plugin = PassPlugin::Load(Filename);
  if (!Plugin)
    return wrap(Plugin.takeError());

  // The callback may use the output stream when it deliberately replaces the
  // normal code-generation pipeline. GoALLC currently only transforms the
  // module, but provide a real pwrite stream so the plugin contract remains
  // complete.
  SmallVector<char, 0> CallbackOutput;
  raw_svector_ostream OS(CallbackOutput);
  Module &M = *unwrap(ModuleRef);
  TargetMachine &TM = *reinterpret_cast<TargetMachine *>(TargetMachineRef);
  if (Plugin->invokePreCodeGenCallback(
          M, TM, static_cast<CodeGenFileType>(FileType), OS)) {
    return LLVMCreateStringError(
        "pass plugin stopped the in-process code-generation pipeline");
  }
  return LLVMErrorSuccess;
}
