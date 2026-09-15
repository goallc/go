// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
#ifndef GOALLC_WRITE_BARRIERS_H
#define GOALLC_WRITE_BARRIERS_H
namespace llvm {
class Module;
namespace goallc {
void configureWriteBarrierRecords(llvm::Module &M);
void lowerWriteBarrierRecords(llvm::Module &M);
} // namespace goallc
} // namespace llvm
#endif
