//go:build !compiler_bootstrap && cgo

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

package llvmbackend

/*
#cgo CPPFLAGS: -I${SRCDIR}/../../vendor/github.com/goallc/go-llvm/llvm/include
#include "../../llvmplugin/GoALLCThinLTOC.h"
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/goallc/go-llvm"
)

func pluginError(err C.LLVMErrorRef) error {
	if err == nil {
		return nil
	}
	message := C.LLVMGetErrorMessage(err)
	defer C.LLVMDisposeErrorMessage(message)
	return fmt.Errorf("ThinLTO plugin: %s", C.GoString(message))
}

func bufferBytes(buffer C.LLVMMemoryBufferRef) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(C.LLVMGetBufferStart(buffer))), C.LLVMGetBufferSize(buffer))
}

// RunThinLTO invokes the GoALLC plugin with in-memory bitcode. Returned slices
// borrow plugin buffers until release is called. Inputs are copied into native
// buffers, which remain alive until every plugin worker has finished.
func RunThinLTO(names []string, inputs [][]byte, plugin, diagnostics string, pic bool, jobs int) ([][]byte, func(), error) {
	if len(inputs) == 0 || len(names) != len(inputs) || jobs <= 0 || int(C.unsigned(jobs)) != jobs {
		return nil, nil, fmt.Errorf("ThinLTO requires named inputs and a positive unsigned job count")
	}
	refs := make([]C.LLVMMemoryBufferRef, len(inputs))
	for i, input := range inputs {
		name := C.CString(names[i])
		refs[i] = C.LLVMCreateMemoryBufferWithMemoryRangeCopy(
			(*C.char)(unsafe.Pointer(unsafe.SliceData(input))), C.size_t(len(input)), name)
		C.free(unsafe.Pointer(name))
		defer C.LLVMDisposeMemoryBuffer(refs[i])
	}
	dir := C.CString(diagnostics)
	defer C.free(unsafe.Pointer(dir))
	var reloc C.LLVMBool
	if pic {
		reloc = 1
	}
	outputs := make([]C.LLVMMemoryBufferRef, len(inputs))
	release := func() {
		for _, output := range outputs {
			if output != nil {
				C.LLVMDisposeMemoryBuffer(output)
			}
		}
	}
	if err := runThinLTO(&refs[0], C.size_t(len(refs)), dir, reloc, C.unsigned(jobs), &outputs[0], plugin); err != nil {
		release()
		return nil, nil, err
	}
	objects := make([][]byte, len(outputs))
	for i, output := range outputs {
		objects[i] = bufferBytes(output)
	}
	return objects, release, nil
}

// WriteThinBitcode asks the GoALLC plugin to write a ThinLTO summary retaining
// Go export identities. The returned bytes are owned by Go.
func WriteThinBitcode(module llvm.Module, plugin string) ([]byte, error) {
	var output C.LLVMMemoryBufferRef
	err := writeThinBitcode(C.LLVMModuleRef(unsafe.Pointer(module.C)), &output, plugin)
	if output != nil {
		defer C.LLVMDisposeMemoryBuffer(output)
	}
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), bufferBytes(output)...), nil
}
