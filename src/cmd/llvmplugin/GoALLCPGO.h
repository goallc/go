// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#ifndef GOALLC_PGO_H
#define GOALLC_PGO_H

#include "llvm/IR/PassManager.h"

namespace llvm {
class Module;
namespace goallc {
class LowerPGOCountersPass
    : public RequiredPassInfoMixin<LowerPGOCountersPass> {
public:
  PreservedAnalyses run(Module &M, ModuleAnalysisManager &);
};
} // namespace goallc
} // namespace llvm

#endif
