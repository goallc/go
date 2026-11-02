//go:build !compiler_bootstrap && cgo && staticllvm && goallcplugin

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
#include "../../llvmplugin/GoALLCThinLTOC.h"

extern char *GoALLCWriteThinBitcode(void *, char **, size_t *);
char *goallcLTO(const char *plugin, const GoALLCLTOInput *inputs, size_t count,
                const char *out, int pic, int jobs, GoALLCLTOBuffer *objects) {
  return GoALLCThinLTOBuffers(inputs, count, out, pic, jobs, objects);
}
char *goallcThinWrite(const char *plugin, void *module, char **data,
                      size_t *size) {
  return GoALLCWriteThinBitcode(module, data, size);
}
