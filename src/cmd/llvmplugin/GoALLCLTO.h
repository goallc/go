// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
#ifndef GOALLC_LTO_H
#define GOALLC_LTO_H
#include "llvm/ADT/ArrayRef.h"
#include "llvm/ADT/StringRef.h"
#include "llvm/Support/Error.h"
namespace goallc {
llvm::Error runLTO(llvm::ArrayRef<std::string> Inputs, llvm::StringRef OutDir,
                   llvm::StringRef Pipeline, bool SaveIR, bool Thin = false);
}
#endif
