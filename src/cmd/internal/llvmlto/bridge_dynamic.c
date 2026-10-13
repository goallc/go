// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
//go:build !compiler_bootstrap && cgo && (!staticllvm || !goallcplugin)
#include <stdlib.h>
#include <string.h>
#include <dlfcn.h>
char *goallcFullLTO(const char *plugin, const char *const *inputs, size_t count, const char *out, int keep) {
 void *handle = dlopen(plugin, RTLD_NOW | RTLD_GLOBAL);
 if (!handle) return strdup(dlerror());
 char *(*run)(const char *const *,size_t,const char *,int) = dlsym(handle,"GoALLCFullLTO");
 if (!run) return strdup(dlerror());
 // LLVM retains registered callbacks. Keep the library loaded until exit.
 return run(inputs,count,out,keep);
}
