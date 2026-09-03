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
#include "llvm/IR/Metadata.h"
#include "llvm/IR/Module.h"
#include "llvm/IR/DiagnosticHandler.h"
#include "llvm/Passes/PassBuilder.h"
#include "llvm/Plugins/PassPlugin.h"
#include "llvm/Support/CBindingWrapping.h"
#include "llvm/Support/DynamicLibrary.h"
#include "llvm/Support/Error.h"
#include "llvm/Support/raw_ostream.h"
#include "llvm/Target/TargetMachine.h"
#include "llvm/TargetParser/Triple.h"
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

static Expected<std::string> getGoObjConfigField(const MDNode &Node,
                                                 unsigned Index) {
  if (const auto *Value = dyn_cast<MDString>(Node.getOperand(Index)))
    return Value->getString().str();
  return createStringError(inconvertibleErrorCode(),
                           "!goobj.config field %u must be a string", Index);
}

LLVMErrorRef LLVMConfigureGoObjFromModule(LLVMModuleRef ModuleRef) {
  const Module &M = *unwrap(ModuleRef);
  const NamedMDNode *Named = M.getNamedMetadata("goobj.config");
  if (!Named)
    return LLVMErrorSuccess;
  if (Named->getNumOperands() != 1)
    return wrap(createStringError(inconvertibleErrorCode(),
                                  "!goobj.config must contain one operand"));

  const MDNode &Node = *Named->getOperand(0);
  if (Node.getNumOperands() != 12)
    return wrap(createStringError(inconvertibleErrorCode(),
                                  "!goobj.config must contain twelve fields"));

  SmallVector<std::string, 11> Fields;
  for (unsigned I = 0; I != 11; ++I) {
    Expected<std::string> Field = getGoObjConfigField(Node, I);
    if (!Field)
      return wrap(Field.takeError());
    Fields.push_back(std::move(*Field));
  }
  const auto *ExperimentNode = dyn_cast<MDNode>(Node.getOperand(11));
  if (!ExperimentNode)
    return wrap(
        createStringError(inconvertibleErrorCode(),
                          "!goobj.config experiments must be a metadata node"));
  SmallVector<std::string, 8> Experiments;
  for (const MDOperand &Operand : ExperimentNode->operands()) {
    const auto *Experiment = dyn_cast<MDString>(Operand.get());
    if (!Experiment)
      return wrap(createStringError(
          inconvertibleErrorCode(),
          "!goobj.config experiment entries must be strings"));
    Experiments.push_back(Experiment->getString().str());
  }

  if (Fields[0] != "goallc.goobj")
    return wrap(createStringError(inconvertibleErrorCode(),
                                  "unsupported !goobj.config schema %s",
                                  Fields[0].c_str()));
  if (Fields[1].empty() || Fields[2].empty() || Fields[3].empty())
    return wrap(createStringError(
        inconvertibleErrorCode(),
        "!goobj.config GOOS, GOARCH, and version must be set"));
  if (Fields[7].empty())
    return wrap(createStringError(inconvertibleErrorCode(),
                                  "!goobj.config package path is empty"));
  if ((Fields[4].empty()) != (Fields[5].empty()))
    return wrap(createStringError(inconvertibleErrorCode(),
                                  "!goobj.config GOARCH setting key and value "
                                  "must be both present or absent"));
  if ((Fields[8] != "0" && Fields[8] != "1") ||
      (Fields[9] != "0" && Fields[9] != "1") ||
      (Fields[10] != "0" && Fields[10] != "1"))
    return wrap(createStringError(
        inconvertibleErrorCode(),
        "!goobj.config main, shared, and std flags must be 0 or 1"));
  if (!Triple(M.getTargetTriple()).isOSBinFormatGoObj())
    return wrap(
        createStringError(inconvertibleErrorCode(),
                          "!goobj.config requires a GoObj target triple"));

  codegen::GoObjConfig Config;
  Config.GOOS = std::move(Fields[1]);
  Config.GOARCH = std::move(Fields[2]);
  Config.Version = std::move(Fields[3]);
  Config.GOARCHSettingKey = std::move(Fields[4]);
  Config.GOARCHSettingValue = std::move(Fields[5]);
  Config.BuildID = std::move(Fields[6]);
  Config.PackagePath = std::move(Fields[7]);
  Config.IsMain = Fields[8] == "1";
  Config.IsShared = Fields[9] == "1";
  Config.IsStd = Fields[10] == "1";
  Config.Experiments.assign(Experiments.begin(), Experiments.end());
  codegen::setGoObjConfig(std::move(Config));
  return LLVMErrorSuccess;
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
