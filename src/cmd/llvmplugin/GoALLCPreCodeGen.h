// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#ifndef GOALLC_PRE_CODEGEN_H
#define GOALLC_PRE_CODEGEN_H

#include "llvm/Support/CodeGen.h"
#include "llvm/Support/Error.h"

namespace llvm {

class Module;
class Function;
class TargetMachine;

namespace goallc {

// Shared preparation for llc and standard LLVM LTO code generation. Run before
// constructing the codegen pipeline, including AsmPrinter initialization.
Error prepareCodeGenModule(Module &M, CodeGenOptLevel OptLevel);

// Runs the GoALLC IR pipeline immediately before LLVM code generation.
//
// The loadable llc plugin and the future in-process compiler integration share
// this entry point so the driver does not determine pass ordering.
Error runPreCodeGenPipeline(Module &M, TargetMachine &TM);

// Owns private byval IR analysis and rewriting in GoALLCByVal.cpp.
// Canonicalizes private aggregate initialization before GC liveness is computed
// so SelectionDAG can forward the final values into a byval argument home.
void prepareByValCallArguments(Function &F);

} // namespace goallc
} // namespace llvm

#endif // GOALLC_PRE_CODEGEN_H
