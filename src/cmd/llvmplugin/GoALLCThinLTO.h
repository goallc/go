// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
#ifndef GOALLC_THIN_LTO_H
#define GOALLC_THIN_LTO_H
#include "llvm/ADT/ArrayRef.h"
#include "llvm/ADT/StringRef.h"
#include "llvm/Support/Error.h"
#include <string>
namespace llvm {
class Module;
class raw_ostream;
} // namespace llvm
namespace goallc {
llvm::Error writeThinBitcode(llvm::Module &, llvm::raw_ostream &);
llvm::Error runThinLTO(llvm::ArrayRef<std::string>, llvm::StringRef, bool,
                       unsigned);
} // namespace goallc
#endif
