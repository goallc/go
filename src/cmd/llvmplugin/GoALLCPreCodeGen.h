// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#ifndef GOALLC_PRE_CODEGEN_H
#define GOALLC_PRE_CODEGEN_H

#include "llvm/Support/Error.h"

namespace llvm {

class Module;
class TargetMachine;

namespace goallc {

// Runs the GoALLC IR pipeline immediately before LLVM code generation.
//
// The loadable llc plugin and the future in-process compiler integration share
// this entry point so the driver does not determine pass ordering.
Error runPreCodeGenPipeline(Module &M, TargetMachine &TM);

} // namespace goallc
} // namespace llvm

#endif // GOALLC_PRE_CODEGEN_H
