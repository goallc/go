// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#ifndef GOALLC_PLUGIN_H
#define GOALLC_PLUGIN_H

namespace llvm {
class PassBuilder;
}

// Use a strong reference for in-process callers. llvmGetPassPluginInfo is a
// weak dynamic-plugin entry point and cannot pull its implementation out of a
// static archive on ELF platforms.
void registerGoALLCPassBuilderCallbacks(llvm::PassBuilder &PB);

#endif // GOALLC_PLUGIN_H
