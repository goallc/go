// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
//go:build !compiler_bootstrap && cgo && staticllvm && goallcplugin
#include <stddef.h>
extern char *GoALLCFullLTO(const char *const *,size_t,const char *,int);
char *goallcFullLTO(const char *plugin, const char *const *inputs, size_t count, const char *out, int keep) {
 return GoALLCFullLTO(inputs,count,out,keep);
}
