// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCPreCodeGen.h"
#include "GoALLCStatepoints.h"
#include "llvm/CodeGen/Analysis.h"
#include "llvm/CodeGen/GoCallingConv.h"
#include "llvm/IR/Dominators.h"
#include "llvm/IR/Instructions.h"
#include "llvm/IR/Module.h"
#include "llvm/Target/TargetMachine.h"

using namespace llvm;

void goallc::prepareByValCallArguments(Function &F) {
  if (F.isDeclaration() || !goabi::isGoCallingConv(F.getCallingConv()) ||
      !F.hasGC() || F.getGC() != "goallc")
    return;
  DominatorTree DT(F);
  for (Instruction &I : F.getEntryBlock()) {
    auto *AI = dyn_cast<AllocaInst>(&I);
    if (!AI || AI->getMetadata("goallc.defer_result") ||
        AI->getMetadata("goallc.open_defer_bits") ||
        AI->getMetadata("goallc.open_defer_slots"))
      continue;
    // Go's defer homes have additional lifetime semantics. All other private
    // byval objects use LLVM's shared preparation before GC liveness, so the
    // GC collector and SelectionDAG see the same carrier ownership proof.
    prepareSingleByValCallCarrier(*AI, DT);
  }
}

Error goallc::runPreCodeGenPipeline(Module &M, TargetMachine &TM) {
  return rewriteStatepoints(M, TM);
}
