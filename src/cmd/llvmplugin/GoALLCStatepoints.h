// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#ifndef GOALLC_STATEPOINTS_H
#define GOALLC_STATEPOINTS_H

#include "llvm/Support/Error.h"

namespace llvm {

class Function;
class Module;
class TargetMachine;

namespace goallc {

// Rewrites calls in Go ABI functions to statepoints using GoALLC's value
// liveness and relocation policy.
Error rewriteStatepoints(Module &M, TargetMachine &TM);

// Performs the module-wide lowering that must precede per-function
// statepoint rewriting but does not itself insert statepoints.
Error prepareStatepointModule(Module &M);

// Rewrites one Go ABI function to statepoints. This entry point is suitable
// for a legacy FunctionPass immediately before instruction selection.
Error rewriteStatepoints(Function &F, TargetMachine &TM);

} // namespace goallc
} // namespace llvm

#endif // GOALLC_STATEPOINTS_H
