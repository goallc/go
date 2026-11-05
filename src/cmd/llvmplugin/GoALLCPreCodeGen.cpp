// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCPreCodeGen.h"
#include "GoALLCStatepoints.h"
#include "llvm/IR/Module.h"
#include "llvm/Target/TargetMachine.h"

using namespace llvm;

Error goallc::runPreCodeGenPipeline(Module &M, TargetMachine &TM) {
  return rewriteStatepoints(M, TM);
}
