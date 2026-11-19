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
#include "llvm-c/Transforms/PassBuilder.h"
#include <stdlib.h>

extern LLVMErrorRef LLVMGoALLCLowerPGOCounters(LLVMModuleRef);
static int configurePGO(LLVMPassBuilderOptionsRef options, const char *profile) {
    LLVMInstrProfLoweringCallback lower = NULL;
    if (!*profile) {
#ifdef GOALLC_LINKED_PLUGIN
        lower = LLVMGoALLCLowerPGOCounters;
#else
        lower = (LLVMInstrProfLoweringCallback)LLVMSearchForAddressOfSymbol("LLVMGoALLCLowerPGOCounters");
        if (!lower)
            return 0;
#endif
    }
    LLVMPassBuilderOptionsSetIRPGO(options, profile, lower);
    return 1;
}
*/
import "C"

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/goallc/go-llvm"
)

// Configure selects LLVM's instrumentation mode. Standard pipelines choose
// their own profiling site. A custom list without one profiles its input.
func Configure(m llvm.Module, options llvm.PassBuilderOptions, pipeline, mode string) (string, error) {
	generate := mode == "generate"
	if generate {
		const versionSymbol = "__llvm_profile_raw_version"
		if !m.NamedGlobal(versionSymbol).IsNil() || !m.NamedFunction(versionSymbol).IsNil() {
			return "", fmt.Errorf("LLVM profile version symbol already defined")
		}
		mode = ""
	}
	profile := C.CString(mode)
	defer C.free(unsafe.Pointer(profile))
	if C.configurePGO(C.LLVMPassBuilderOptionsRef(unsafe.Pointer(options.C)), profile) == 0 {
		return "", fmt.Errorf("GoALLC plugin lacks PGO counter lowering")
	}
	if pipeline == "none" {
		pipeline = ""
	}
	var sites, gen, lower, use int
	for _, token := range strings.FieldsFunc(pipeline, func(r rune) bool { return r == '(' || r == ')' || r == ',' }) {
		name, _, _ := strings.Cut(strings.TrimSpace(token), "<")
		switch name {
		case "default", "lto-pre-link", "thinlto-pre-link", "fatlto-pre-link":
			sites++
		case "pgo-instr-gen":
			gen++
		case "instrprof":
			if gen != 1 {
				return "", fmt.Errorf("instrprof requires preceding pgo-instr-gen in %q", pipeline)
			}
			lower++
		case "pgo-instr-use":
			use++
		}
	}
	if sites+gen+use > 1 || gen != lower || generate && use != 0 || !generate && gen != 0 {
		return "", fmt.Errorf("conflicting LLVM PGO passes in %q", pipeline)
	}
	if sites+gen+use != 0 {
		return pipeline, nil
	}
	prefix := "pgo-instr-use"
	if generate {
		prefix = "pgo-instr-gen,instrprof"
	}
	if pipeline != "" {
		prefix += "," + pipeline
	}
	return prefix, nil
}
