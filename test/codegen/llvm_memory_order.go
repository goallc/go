// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

var llvmMemoryOrderRoot *int

//go:noinline
func llvmMemoryOrderStore(value *int) {
	llvmMemoryOrderRoot = value
}

// LLVM-LABEL: define goabiinternal ptr @codegen.llvmMemoryOrderLoad(
// LLVM: call goabiinternal void @codegen.llvmMemoryOrderStore(
// LLVM: load ptr, ptr @codegen.llvmMemoryOrderRoot
// LLVM: ret ptr
// LLVM-OPT-LABEL: define goabiinternal ptr @codegen.llvmMemoryOrderLoad(
// LLVM-OPT-SAME: ptr{{[^%]*}}%[[VALUE:[a-zA-Z0-9._]+]])
// LLVM-OPT: call ptr @llvm.go.gc.write.barrier(i32 2)
// LLVM-OPT: store ptr %[[VALUE]], ptr @codegen.llvmMemoryOrderRoot
// LLVM-OPT: ret ptr %[[VALUE]]
func llvmMemoryOrderLoad(value *int) *int {
	llvmMemoryOrderStore(value)
	return llvmMemoryOrderRoot
}
