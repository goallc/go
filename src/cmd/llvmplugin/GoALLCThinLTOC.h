// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
#ifndef GOALLC_THIN_LTO_C_H
#define GOALLC_THIN_LTO_C_H
#include <stddef.h>
#ifdef __cplusplus
extern "C" {
#endif
// Inputs remain borrowed for the duration of the synchronous call. Names are
// unique module identifiers; they need not name files on disk.
typedef struct {
  const char *name;
  const char *data;
  size_t size;
} GoALLCLTOInput;
// Caller zero-initializes outputs and frees every data pointer with free(),
// including partial outputs when the call returns an error.
typedef struct {
  char *data;
  size_t size;
} GoALLCLTOBuffer;
// Empty diagnostics disables all intermediate files. Errors are malloc-owned.
char *GoALLCThinLTOBuffers(const GoALLCLTOInput *, size_t, const char *, int,
                           int, GoALLCLTOBuffer *);
#ifdef __cplusplus
}
#endif
#endif
