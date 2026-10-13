//go:build !compiler_bootstrap && cgo && staticllvm && goallcplugin

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
#include <stddef.h>
extern char *GoALLCLTO(const char *const *, size_t, const char *, int, int);
extern char *GoALLCFullLTOSingle(const char *const *, size_t, const char *, int);
char *goallcLTO(const char *plugin, const char *const *inputs, size_t count,
                const char *out, int keep, int thin) {
  return thin ? GoALLCLTO(inputs, count, out, keep, thin)
              : GoALLCFullLTOSingle(inputs, count, out, keep);
}
