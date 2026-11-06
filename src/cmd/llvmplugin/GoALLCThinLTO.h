// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
#ifndef GOALLC_THIN_LTO_H
#define GOALLC_THIN_LTO_H
#include "llvm/ADT/ArrayRef.h"
#include "llvm/ADT/STLFunctionalExtras.h"
#include "llvm/ADT/StringRef.h"
#include "llvm/Support/Error.h"
#include "llvm/Support/MemoryBufferRef.h"
namespace llvm {
class Module;
class raw_ostream;
} // namespace llvm
namespace goallc {
llvm::Error writeThinBitcode(llvm::Module &, llvm::raw_ostream &);
// Inputs and their unique identifiers remain alive until all workers finish.
// Consume runs in each worker while its optimized Module and Context are alive.
llvm::Error
runThinLTO(llvm::ArrayRef<llvm::MemoryBufferRef>, llvm::StringRef, unsigned,
           llvm::function_ref<llvm::Error(unsigned, llvm::Module &)> Consume);
} // namespace goallc
#endif
