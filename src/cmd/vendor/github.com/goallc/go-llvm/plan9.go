//===- plan9.go - Deferred Plan 9 assembler bindings -----------------------===//
// Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package llvm

/*
#include <stdlib.h>
#include <stddef.h>
extern void LLVMGoSetPlan9Assembler(const char *(*)(const char *, size_t),
                                   void (*)(const char *));
extern char *goLLVMPlan9Assemble(char *, size_t);
static const char *callPlan9(const char *p, size_t n) {
  return goLLVMPlan9Assemble((char *)p, n);
}
static void freePlan9(const char *p) { free((void *)p); }
static void enablePlan9(void) { LLVMGoSetPlan9Assembler(callPlan9, freePlan9); }
static void disablePlan9(void) { LLVMGoSetPlan9Assembler(NULL, NULL); }
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

var plan9Mu sync.Mutex
var plan9Assembler func([]byte) ([]byte, error)

// WithPlan9Assembler installs the deferred assembler for one synchronous codegen
// invocation. LLVM stores the hook on the calling OS thread. Serialized entry
// also protects the Go assembler's process-wide build configuration.
func WithPlan9Assembler(assemble func([]byte) ([]byte, error), emit func() error) error {
	plan9Mu.Lock()
	defer plan9Mu.Unlock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	plan9Assembler = assemble
	C.enablePlan9()
	defer func() { C.disablePlan9(); plan9Assembler = nil }()
	return emit()
}

//export goLLVMPlan9Assemble
func goLLVMPlan9Assemble(p *C.char, n C.size_t) (result *C.char) {
	fail := func(err any) *C.char {
		data, _ := json.Marshal(map[string]string{"error": fmt.Sprint(err)})
		return C.CString(string(data))
	}
	defer func() {
		if err := recover(); err != nil {
			result = fail(err)
		}
	}()
	if uint64(n) > uint64(^uint32(0)>>1) {
		return fail("Plan 9 input too large")
	}
	data, err := plan9Assembler(C.GoBytes(unsafe.Pointer(p), C.int(n)))
	if err != nil {
		return fail(err)
	}
	return C.CString(string(data))
}
