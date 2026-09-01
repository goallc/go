//===- SupportBindings.h - Additional bindings for Support ------*- C++ -*-===//
//
// Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
// See https://llvm.org/LICENSE.txt for license information.
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception
//
//===----------------------------------------------------------------------===//
//
// This file defines additional C bindings for the Support component.
//
//===----------------------------------------------------------------------===//

#ifndef LLVM_BINDINGS_GO_LLVM_SUPPORTBINDINGS_H
#define LLVM_BINDINGS_GO_LLVM_SUPPORTBINDINGS_H

#include "llvm-c/Core.h"
#include "llvm-c/Error.h"
#include "llvm-c/TargetMachine.h"

#ifdef __cplusplus
extern "C" {
#endif

// This function duplicates the LLVMLoadLibraryPermanently function in the
// stable C API and adds an extra ErrMsg parameter to retrieve the error
// message.
void LLVMLoadLibraryPermanently2(const char *Filename, char **ErrMsg);

// Decode the Go frontend's !goobj.config metadata into the LLVM GoObj writer
// configuration used by the target code-generation pipeline.
LLVMErrorRef LLVMConfigureGoObjFromModule(LLVMModuleRef Module);

// Load a pass plugin and run its named GoALLC early-IR pipeline. This happens
// before the caller's normal optimization pipeline.
LLVMErrorRef LLVMRunPassPluginEarlyIR(LLVMModuleRef Module,
                                      const char *Filename);

// Load a pass plugin and invoke its pre-codegen callback on Module. This is
// the in-process equivalent of llc -load-pass-plugin immediately before llc
// constructs the target code-generation pipeline.
LLVMErrorRef LLVMRunPassPluginPreCodeGen(LLVMModuleRef Module,
                                         LLVMTargetMachineRef TargetMachine,
                                         LLVMCodeGenFileType FileType,
                                         const char *Filename);

#ifdef __cplusplus
}
#endif

#endif
