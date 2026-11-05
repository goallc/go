// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCPreCodeGen.h"
#include "GoALLCStatepoints.h"
#include "llvm/IR/Module.h"
#include "llvm/Target/TargetMachine.h"

using namespace llvm;

void goallc::configureFrameBasePreservation(Module &M) {
  // SROA must account for the cost of relocating movable stack addresses.
  // Retain one frame/object base merge rather than splitting it into an
  // independent base/offset pair for every field live across calls.
  for (Function &F : M)
    if (!F.isDeclaration() && F.hasGC() && F.getGC() == "goallc")
      F.addFnAttr("gc-preserve-frame-base");
}

Error goallc::runPreCodeGenPipeline(Module &M, TargetMachine &TM) {
  return rewriteStatepoints(M, TM);
}
