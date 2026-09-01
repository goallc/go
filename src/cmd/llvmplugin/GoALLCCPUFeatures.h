// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#ifndef GOALLC_CPU_FEATURES_H
#define GOALLC_CPU_FEATURES_H

#include "llvm/IR/PassManager.h"
#include "llvm/Support/Error.h"

namespace llvm {

class Function;
class Module;

namespace goallc {

// Runs GoALLC function multiversioning before the normal LLVM optimization
// pipeline. The transformation is module-idempotent so no-opt llc paths may
// safely invoke it again from the pre-codegen callback.
Error runEarlyIRPipeline(Module &M);

// Promotes calls in marked FMV functions which remain terminal after
// optimization to musttail immediately before statepoint rewriting and
// instruction selection. Calls made non-terminal by inlining keep ordinary
// safepoint semantics.
Error finalizeCPUFeatureTailTransfers(Function &F);

class CPUFeaturesPass : public PassInfoMixin<CPUFeaturesPass> {
public:
  PreservedAnalyses run(Module &M, ModuleAnalysisManager &AM);
};

} // namespace goallc
} // namespace llvm

#endif // GOALLC_CPU_FEATURES_H
