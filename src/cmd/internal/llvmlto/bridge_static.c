//go:build !compiler_bootstrap && cgo && staticllvm && goallcplugin

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
#include <stddef.h>
extern char *GoALLCThinLTO(const char *const *, size_t, const char *, int, int);
extern char *GoALLCWriteThinBitcode(void *, char **, size_t *);
char *goallcLTO(const char *plugin, const char *const *inputs, size_t count,
               const char *out, int keep, int jobs) {
  return GoALLCThinLTO(inputs, count, out, keep, jobs);
}
char *goallcThinWrite(const char *plugin, void *module, char **data, size_t *size) {
  return GoALLCWriteThinBitcode(module, data, size);
}
