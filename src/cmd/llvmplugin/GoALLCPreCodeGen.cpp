// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCPreCodeGen.h"
#include "llvm/IR/Module.h"
#include "llvm/Target/TargetMachine.h"

using namespace llvm;

Error goallc::runPreCodeGenPipeline(Module &M, TargetMachine &TM) {
  // The statepoint rewrite will be added here. Keep this core independent of
  // the llc plugin entry point so it can later be linked into cmd/compile.
  (void)M;
  (void)TM;
  return Error::success();
}
