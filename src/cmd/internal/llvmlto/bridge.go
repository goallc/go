//go:build !compiler_bootstrap && cgo

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
package llvmlto

/*
#include <stdlib.h>
#cgo linux LDFLAGS: -ldl
char *goallcLTO(const char *, const char *const *, size_t, const char *, int, int);
char *goallcThinWrite(const char *, void *, char **, size_t *);
*/
import "C"
import (
	"cmd/internal/llvmbackend"
	"fmt"
	"unsafe"

	"github.com/goallc/go-llvm"
)

func optimize(inputs []string, out string, keep bool, jobs int) error {
	plugin := ""
	if !llvm.UsesLinkedPassPlugin() {
		var err error
		plugin, err = llvmbackend.PassPlugin()
		if err != nil {
			return err
		}
	}
	path := C.CString(plugin)
	defer C.free(unsafe.Pointer(path))
	dir := C.CString(out)
	defer C.free(unsafe.Pointer(dir))
	values := make([]*C.char, len(inputs))
	for i, input := range inputs {
		values[i] = C.CString(input)
		defer C.free(unsafe.Pointer(values[i]))
	}
	save := C.int(0)
	if keep {
		save = 1
	}
	message := C.goallcLTO(path, (**C.char)(unsafe.Pointer(&values[0])), C.size_t(len(values)), dir, save, C.int(jobs))
	if message != nil {
		defer C.free(unsafe.Pointer(message))
		return fmt.Errorf("LTO: %s", C.GoString(message))
	}
	return nil
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
