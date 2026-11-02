//go:build !compiler_bootstrap && cgo && (!staticllvm || !goallcplugin)

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
#include "../../llvmplugin/GoALLCThinLTOC.h"
#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>

char *goallcLTO(const char *plugin, const GoALLCLTOInput *inputs, size_t count,
                const char *out, int pic, int jobs, GoALLCLTOBuffer *objects) {
  void *handle = dlopen(plugin, RTLD_NOW | RTLD_GLOBAL);
  if (!handle)
    return strdup(dlerror());
  char *(*run)(const GoALLCLTOInput *, size_t, const char *, int, int,
               GoALLCLTOBuffer *) = dlsym(handle, "GoALLCThinLTOBuffers");
  if (!run)
    return strdup(dlerror());
  // LLVM retains registered callbacks. Keep the library loaded until exit.
  return run(inputs, count, out, pic, jobs, objects);
}

char *goallcThinWrite(const char *plugin, void *module, char **data,
                      size_t *size) {
  void *handle = dlopen(plugin, RTLD_NOW | RTLD_GLOBAL);
  if (!handle)
    return strdup(dlerror());
  char *(*write)(void *, char **, size_t *) =
      dlsym(handle, "GoALLCWriteThinBitcode");
  if (!write)
    return strdup(dlerror());
  return write(module, data, size);
}
