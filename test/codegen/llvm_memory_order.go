// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "runtime"

var llvmMemoryOrderRoot *int

func llvmSemanticKeepAlive(value *int) {
	runtime.KeepAlive(value)
}

//go:noinline
func llvmMemoryOrderStore(value *int) {
	llvmMemoryOrderRoot = value
}

// LLVM-LABEL: define goabiinternal ptr @codegen.llvmMemoryOrderLoad(
// LLVM: call goabiinternal void @codegen.llvmMemoryOrderStore(
// LLVM: load ptr, ptr @codegen.llvmMemoryOrderRoot
// LLVM: ret ptr
// LLVM-LABEL: define goabiinternal void @codegen.llvmMemoryOrderStore(
// LLVM-SAME: #[[NOINLINE_ATTR:[0-9]+]] gc "goallc"
// LLVM-OPT-LABEL: define goabiinternal ptr @codegen.llvmMemoryOrderLoad(
// LLVM-OPT-SAME: ptr{{[^%]*}}%[[VALUE:[a-zA-Z0-9._]+]])
// LLVM-OPT: call goabiinternal void @codegen.llvmMemoryOrderStore(ptr %[[VALUE]])
// LLVM-OPT: %[[ROOT:[a-zA-Z0-9._]+]] = load ptr, ptr @codegen.llvmMemoryOrderRoot
// LLVM-OPT: ret ptr %[[ROOT]]
// LLVM-OPT-LABEL: define goabiinternal void @codegen.llvmMemoryOrderStore(
// LLVM-OPT-SAME: #[[NOINLINE_ATTR:[0-9]+]] gc "goallc"
// LLVM-OPT: call void @goallc.gc.write.record(ptr %[[VALUE]], ptr {{(nonnull )?}}@codegen.llvmMemoryOrderRoot, i32 0)
// LLVM-OPT: store ptr %[[VALUE]], ptr @codegen.llvmMemoryOrderRoot
// LLVM-LABEL: define goabiinternal void @codegen.llvmSemanticKeepAlive(
// LLVM: call void @llvm.donothing() [ "go.keepalive"(ptr %{{.*}}) ]
// LLVM-OPT-LABEL: define goabiinternal void @codegen.llvmSemanticKeepAlive(
// LLVM-OPT: call void @llvm.donothing() [ "go.keepalive"(ptr %{{.*}}) ]
// LLVM: attributes #[[NOINLINE_ATTR]] = { {{.*}}noinline
// LLVM-OPT: attributes #[[NOINLINE_ATTR]] = { {{.*}}noinline
func llvmMemoryOrderLoad(value *int) *int {
	llvmMemoryOrderStore(value)
	return llvmMemoryOrderRoot
}
