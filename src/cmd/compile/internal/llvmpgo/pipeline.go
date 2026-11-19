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

extern LLVMErrorRef LLVMGoALLCLowerPGOCounters(LLVMModuleRef);
static void *counterLowering(void) {
#ifdef GOALLC_LINKED_PLUGIN
	return (void *)LLVMGoALLCLowerPGOCounters;
#else
	return LLVMSearchForAddressOfSymbol("LLVMGoALLCLowerPGOCounters");
#endif
}
*/
import "C"

import (
	"errors"

	"github.com/goallc/go-llvm"
)

// Run passes the selected pipeline to LLVM with the GoObj profile runtime.
func Run(m llvm.Module, tm llvm.TargetMachine, pipeline, mode string) error {
	if pipeline == "none" {
		pipeline = ""
	}
	var lower llvm.InstrProfLowering
	if mode == "generate" {
		const versionSymbol = "__llvm_profile_raw_version"
		if !m.NamedGlobal(versionSymbol).IsNil() || !m.NamedFunction(versionSymbol).IsNil() {
			return errors.New("LLVM profile version symbol already defined")
		}
		lower = llvm.InstrProfLowering(C.counterLowering())
		if lower == nil {
			return errors.New("GoALLC plugin lacks PGO counter lowering")
		}
		mode = ""
	}
	return m.RunPassesWithPGO(pipeline, tm, mode, lower)
}
