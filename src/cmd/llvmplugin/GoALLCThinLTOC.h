// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
#ifndef GOALLC_THIN_LTO_C_H
#define GOALLC_THIN_LTO_C_H

#include "llvm-c/Core.h"
#include "llvm-c/Error.h"

#ifdef __cplusplus
extern "C" {
#endif

// Inputs are borrowed until the synchronous call returns. Callers initialize
// Outputs to null and dispose every non-null buffer, including partial results
// on failure, with LLVMDisposeMemoryBuffer. Buffer identifiers must be unique.
LLVMErrorRef LLVMGoALLCThinLTO(LLVMMemoryBufferRef *Inputs, size_t Count,
                             const char *Diagnostics, LLVMBool PIC,
                             unsigned Jobs, LLVMMemoryBufferRef *Outputs);
LLVMErrorRef LLVMGoALLCWriteThinBitcode(LLVMModuleRef Module,
                                     LLVMMemoryBufferRef *Output);

#ifdef __cplusplus
}
#endif
#endif
