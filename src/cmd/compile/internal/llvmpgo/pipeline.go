// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

// Package llvmpgo configures IR profiling for the compiler's LLVM pipeline.
package llvmpgo

/*
#cgo CPPFLAGS: -I${SRCDIR}/../../../vendor/github.com/goallc/go-llvm/llvm/include
#cgo staticllvm CPPFLAGS: -DGOALLC_LINKED_PLUGIN
#include "llvm-c/Support.h"
#include "llvm-c/Error.h"

extern LLVMErrorRef LLVMGoALLCAdaptPGOData(LLVMModuleRef);
static LLVMErrorRef adapt(LLVMModuleRef m) {
#ifdef GOALLC_LINKED_PLUGIN
    return LLVMGoALLCAdaptPGOData(m);
#else
    typedef LLVMErrorRef (*Adapt)(LLVMModuleRef);
    Adapt fn = (Adapt)LLVMSearchForAddressOfSymbol("LLVMGoALLCAdaptPGOData");
    if (!fn) return LLVMCreateStringError("GoALLC plugin lacks PGO data adapter");
    return fn(m);
#endif
}
*/
import "C"

import (
	"errors"
	"unsafe"

	"github.com/goallc/go-llvm"
)

// Run passes the selected pipeline to LLVM with the GoObj profile runtime.
func Run(m llvm.Module, tm llvm.TargetMachine, pipeline, mode string) error {
	if pipeline == "none" {
		pipeline = ""
	}
	generate := mode == "generate"
	if generate {
		const versionSymbol = "__llvm_profile_raw_version"
		if !m.NamedGlobal(versionSymbol).IsNil() || !m.NamedFunction(versionSymbol).IsNil() {
			return errors.New("LLVM profile version symbol already defined")
		}
		mode = ""
	}
	if err := m.RunPassesWithPGO(pipeline, tm, mode); err != nil || !generate {
		return err
	}
	if err := C.adapt(C.LLVMModuleRef(unsafe.Pointer(m.C))); err != nil {
		msg := C.LLVMGetErrorMessage(err)
		defer C.LLVMDisposeErrorMessage(msg)
		return errors.New(C.GoString(msg))
	}
	return nil
}
