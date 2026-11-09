// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
#ifndef GOALLC_THIN_LTO_H
#define GOALLC_THIN_LTO_H
#include "llvm/ADT/ArrayRef.h"
#include "llvm/ADT/StringRef.h"
#include "llvm/Support/Caching.h"
#include "llvm/Support/Error.h"
#include "llvm/Support/MemoryBufferRef.h"
namespace llvm {
class Module;
class raw_ostream;
namespace goallc {
Error writeThinBitcode(Module &, raw_ostream &);
// Inputs and their unique identifiers remain alive until all workers finish.
// An empty AddStream requests optimized IR only, for diagnostic tests.
Error runThinLTO(ArrayRef<MemoryBufferRef>, StringRef, unsigned Jobs,
                 bool PIC = false, AddStreamFn AddStream = {});
} // namespace goallc
} // namespace llvm
#endif
