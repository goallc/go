// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

//go:noescape
func deadAutoObserve(*[128]byte)

// Remove unread storage before LLVM has to analyze and promote it.
// LLVM-LABEL: define goabiinternal i64 @codegen.deadAutoStore(
// LLVM-NOT: alloca
// LLVM-NOT: @llvm.memset
// LLVM-NOT: store
// LLVM: ret i64 128

// An address passed to a logical call must retain its initialized storage.
// LLVM-LABEL: define goabiinternal void @codegen.liveAutoCall(
// LLVM: alloca [128 x i8]
// LLVM: call void @llvm.memset
// LLVM: call goabiinternal void @codegen.deadAutoObserve(ptr
// LLVM: ret void

//go:noinline
func deadAutoStore(index int) int {
	var local [128]byte
	local[index&127] = 1
	return len(local)
}

//go:noinline
func liveAutoCall() {
	var local [128]byte
	deadAutoObserve(&local)
}
