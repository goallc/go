//go:build !compiler_bootstrap && cgo

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
package llvmlto

/*
#include <stdlib.h>
#include "../../llvmplugin/GoALLCThinLTOC.h"
#cgo linux LDFLAGS: -ldl
char *goallcLTO(const char *, const GoALLCLTOInput *, size_t, const char *, int, int, GoALLCLTOBuffer *);
char *goallcThinWrite(const char *, void *, char **, size_t *);
*/
import "C"
import (
	"cmd/internal/llvmbackend"
	"fmt"
	"sync"
	"unsafe"

	"github.com/goallc/go-llvm"
)

var initialize sync.Once

// Output slices borrow C allocations until release. Inputs are copied into C
// memory so LLVM workers can retain their buffers throughout importing.
func optimize(names []string, inputs [][]byte, diagnostics string, pic bool, jobs int, arch string) ([][]byte, func(), error) {
	initialize.Do(func() {
		llvm.ParseCommandLineOptions(append([]string{"cmd/link LTO"}, llvmbackend.CodeGenOptions(arch)...), "GoALLC LTO")
	})
	plugin := ""
	if !llvm.UsesLinkedPassPlugin() {
		var err error
		plugin, err = llvmbackend.PassPlugin()
		if err != nil {
			return nil, nil, err
		}
	}
	path := C.CString(plugin)
	defer C.free(unsafe.Pointer(path))
	dir := C.CString(diagnostics)
	defer C.free(unsafe.Pointer(dir))
	values := make([]C.GoALLCLTOInput, len(inputs))
	for i, input := range inputs {
		values[i].name = C.CString(names[i])
		values[i].data = (*C.char)(C.CBytes(input))
		values[i].size = C.size_t(len(input))
		defer C.free(unsafe.Pointer(values[i].name))
		defer C.free(unsafe.Pointer(values[i].data))
	}
	outputs := make([]C.GoALLCLTOBuffer, len(inputs))
	release := func() {
		for _, output := range outputs {
			C.free(unsafe.Pointer(output.data))
		}
	}
	var reloc C.int
	if pic {
		reloc = 1
	}
	message := C.goallcLTO(path, &values[0], C.size_t(len(values)), dir, reloc, C.int(jobs), &outputs[0])
	if message != nil {
		defer C.free(unsafe.Pointer(message))
		release()
		return nil, nil, fmt.Errorf("LTO: %s", C.GoString(message))
	}
	objects := make([][]byte, len(outputs))
	for i, output := range outputs {
		if uint64(output.size) > uint64(^uint(0)>>1) {
			release()
			return nil, nil, fmt.Errorf("ThinLTO object exceeds addressable memory")
		}
		objects[i] = unsafe.Slice((*byte)(unsafe.Pointer(output.data)), int(output.size))
	}
	return objects, release, nil
}

// WriteThinBitcode writes LLVM summaries with the Go export identities needed
// when imported code introduces references absent from the original package.
func WriteThinBitcode(m llvm.Module) ([]byte, error) {
	plugin := ""
	if !llvm.UsesLinkedPassPlugin() {
		var err error
		plugin, err = llvmbackend.PassPlugin()
		if err != nil {
			return nil, err
		}
	}
	path := C.CString(plugin)
	defer C.free(unsafe.Pointer(path))
	var data *C.char
	var size C.size_t
	message := C.goallcThinWrite(path, unsafe.Pointer(m.C), &data, &size)
	if message != nil {
		defer C.free(unsafe.Pointer(message))
		return nil, fmt.Errorf("ThinLTO bitcode: %s", C.GoString(message))
	}
	defer C.free(unsafe.Pointer(data))
	if uint64(size) > uint64(^uint(0)>>1) {
		return nil, fmt.Errorf("ThinLTO bitcode exceeds addressable memory")
	}
	return append([]byte(nil), unsafe.Slice((*byte)(unsafe.Pointer(data)), int(size))...), nil
}
