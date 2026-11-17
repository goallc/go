//go:build !compiler_bootstrap && cgo && (!staticllvm || !goallcplugin)

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

package llvmbackend

/*
#cgo linux LDFLAGS: -ldl
#include "../../llvmplugin/GoALLCThinLTOC.h"
#include <dlfcn.h>
#include <stdlib.h>

static LLVMErrorRef goallcPluginSymbol(const char *plugin, const char *name,
                                      void **symbol) {
  void *handle = dlopen(plugin, RTLD_NOW | RTLD_GLOBAL);
  if (!handle)
    return LLVMCreateStringError(dlerror());
  *symbol = dlsym(handle, name);
  if (!*symbol)
    return LLVMCreateStringError(dlerror());
  // LLVM retains registered callbacks. Keep the library loaded until exit.
  return LLVMErrorSuccess;
}

static LLVMErrorRef goallcThinLTO(const char *plugin,
                                 LLVMMemoryBufferRef *inputs, size_t count,
                                 const char *diagnostics, LLVMBool pic,
                                 unsigned jobs, LLVMMemoryBufferRef *outputs) {
  void *symbol;
  LLVMErrorRef err = goallcPluginSymbol(plugin, "LLVMGoALLCThinLTO", &symbol);
  if (err)
    return err;
  return ((LLVMErrorRef (*)(LLVMMemoryBufferRef *, size_t, const char *,
                           LLVMBool, unsigned, LLVMMemoryBufferRef *))symbol)(
      inputs, count, diagnostics, pic, jobs, outputs);
}

static LLVMErrorRef goallcWriteThinBitcode(const char *plugin,
                                         LLVMModuleRef module,
                                         LLVMMemoryBufferRef *output) {
  void *symbol;
  LLVMErrorRef err = goallcPluginSymbol(plugin, "LLVMGoALLCWriteThinBitcode", &symbol);
  if (err)
    return err;
  return ((LLVMErrorRef (*)(LLVMModuleRef, LLVMMemoryBufferRef *))symbol)(module, output);
}
*/
import "C"

import "unsafe"

func runThinLTO(inputs *C.LLVMMemoryBufferRef, count C.size_t, diagnostics *C.char, pic C.LLVMBool, jobs C.unsigned, outputs *C.LLVMMemoryBufferRef, plugin string) error {
	path := C.CString(plugin)
	defer C.free(unsafe.Pointer(path))
	return pluginError(C.goallcThinLTO(path, inputs, count, diagnostics, pic, jobs, outputs))
}

func writeThinBitcode(module C.LLVMModuleRef, output *C.LLVMMemoryBufferRef, plugin string) error {
	path := C.CString(plugin)
	defer C.free(unsafe.Pointer(path))
	return pluginError(C.goallcWriteThinBitcode(path, module, output))
}
