//go:build !compiler_bootstrap && cgo

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
package llvmlto

/*
#include <stdlib.h>
#cgo linux LDFLAGS: -ldl
char *goallcLTO(const char *, const char *const *, size_t, const char *, int, int);
*/
import "C"
import (
	"cmd/internal/llvmbackend"
	"fmt"
	"github.com/goallc/go-llvm"
	"strings"
	"unsafe"
)

func optimize(inputs []string, out string, keep bool, mode string) error {
	if keep {
		for _, input := range inputs {
			c := llvm.NewContext()
			m, err := c.ParseBitcodeFile(input)
			if err != nil {
				c.Dispose()
				return err
			}
			err = llvm.LLVMPrintModuleToFile(m, strings.TrimSuffix(input, ".bc")+".ll")
			m.Dispose()
			c.Dispose()
			if err != nil {
				return err
			}
		}
	}
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
	thin := C.int(0)
	if mode == "thin" {
		thin = 1
	}
	message := C.goallcLTO(path, (**C.char)(unsafe.Pointer(&values[0])), C.size_t(len(values)), dir, save, thin)
	if message != nil {
		defer C.free(unsafe.Pointer(message))
		return fmt.Errorf("LTO: %s", C.GoString(message))
	}
	return nil
}
