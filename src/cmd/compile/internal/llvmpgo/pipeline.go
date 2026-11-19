// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

// Package llvmpgo invokes the GoObj-aware pipeline in the compiler's LLVM plugin.
package llvmpgo

/*
#cgo CPPFLAGS: -I${SRCDIR}/../../../vendor/github.com/goallc/go-llvm/llvm/include
#cgo staticllvm CPPFLAGS: -DGOALLC_LINKED_PLUGIN
#include "llvm-c/Error.h"
#include "llvm-c/Support.h"
#include "llvm-c/TargetMachine.h"
#include <stdlib.h>

extern LLVMErrorRef LLVMGoALLCRunPGOPipeline(LLVMModuleRef, LLVMTargetMachineRef, const char *, const char *);
static LLVMErrorRef runPipeline(LLVMModuleRef m, LLVMTargetMachineRef tm, const char *passes, const char *mode) {
#ifdef GOALLC_LINKED_PLUGIN
  return LLVMGoALLCRunPGOPipeline(m, tm, passes, mode);
#else
  // Early IR preparation has already loaded the plugin into LLVM.
  typedef LLVMErrorRef (*Run)(LLVMModuleRef, LLVMTargetMachineRef, const char *, const char *);
  Run run = (Run)LLVMSearchForAddressOfSymbol("LLVMGoALLCRunPGOPipeline");
  if (!run)
    return LLVMCreateStringError("GoALLC plugin lacks its PGO pipeline entry point");
  return run(m, tm, passes, mode);
#endif
}
*/
import "C"

import (
	"errors"
	"unsafe"

	"github.com/goallc/go-llvm"
)

func Run(m llvm.Module, tm llvm.TargetMachine, pipeline, mode string) error {
	passes := C.CString(pipeline)
	defer C.free(unsafe.Pointer(passes))
	profile := C.CString(mode)
	defer C.free(unsafe.Pointer(profile))
	err := C.runPipeline(C.LLVMModuleRef(unsafe.Pointer(m.C)), C.LLVMTargetMachineRef(unsafe.Pointer(tm.C)), passes, profile)
	if err == nil {
		return nil
	}
	message := C.LLVMGetErrorMessage(err)
	defer C.LLVMDisposeErrorMessage(message)
	return errors.New(C.GoString(message))
}
