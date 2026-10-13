//go:build !compiler_bootstrap && cgo && (!staticllvm || !goallcplugin)

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>
char *goallcLTO(const char *plugin, const char *const *inputs, size_t count,
                const char *out, int keep, int thin) {
  void *handle = dlopen(plugin, RTLD_NOW | RTLD_GLOBAL);
  if (!handle)
    return strdup(dlerror());
  char *(*run)(const char *const *, size_t, const char *, int, int) =
      dlsym(handle, "GoALLCLTO");
  if (!run)
    return strdup(dlerror());
  // LLVM retains registered callbacks. Keep the library loaded until exit.
  return run(inputs, count, out, keep, thin);
}
