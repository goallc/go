// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

var llvmEscapedPointers *[32]*int

// VarDef for the second result separates the first result's memory tokens.
// The snapshot is correct in unoptimized IR, but copying an unchanged result
// home to a temporary and back must disappear with optimization.
//
// LLVM-LABEL: define goabiinternal void @codegen.llvmSnapshotReturn(
// LLVM: .snapshot = alloca [65536 x i8]
// LLVM: call void @llvm.memmove.p0.p0.i64({{.*}}i64 65536, i1 false)
// LLVM: call void @llvm.memmove.p0.p0.i64({{.*}}i64 65536, i1 false)
// LLVM: ret void
// LLVM-OPT-LABEL: define goabiinternal void @codegen.llvmSnapshotReturn(
// LLVM-OPT-NOT: alloca [65536 x i8]
// LLVM-OPT-NOT: .snapshot
// LLVM-OPT-NOT: @llvm.mem{{cpy|move}}{{.*}}i64 65536
// LLVM-OPT-NOT: @runtime.memmove
// LLVM-OPT: ret void
//
//go:noinline
func llvmSnapshotReturn() (a [1 << 16]byte, p [32]*int) {
	for i := range a {
		a[i] = byte(i)
	}
	llvmEscapedPointers = &p
	p[0] = new(int)
	*p[0] = 42
	return
}
