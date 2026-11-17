//go:build !compiler_bootstrap && cgo && staticllvm && goallcplugin

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

package llvmbackend

/*
#include "../../llvmplugin/GoALLCThinLTOC.h"
*/
import "C"

import "fmt"

func runThinLTO(inputs *C.LLVMMemoryBufferRef, count C.size_t, diagnostics *C.char, pic C.LLVMBool, jobs C.unsigned, outputs *C.LLVMMemoryBufferRef, plugin string) error {
	if plugin != "" {
		return fmt.Errorf("linked GoALLC ThinLTO callback does not accept a plugin path")
	}
	return pluginError(C.LLVMGoALLCThinLTO(inputs, count, diagnostics, pic, jobs, outputs))
}

func writeThinBitcode(module C.LLVMModuleRef, output *C.LLVMMemoryBufferRef, plugin string) error {
	if plugin != "" {
		return fmt.Errorf("linked GoALLC ThinLTO bitcode callback does not accept a plugin path")
	}
	return pluginError(C.LLVMGoALLCWriteThinBitcode(module, output))
}
