// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM: @codegen..typeAssert.0 = internal global <{ ptr, ptr, [8 x i8] }>
// LLVM-LABEL: define goabiinternal { i64, ptr } @codegen.convertLLVMInterface(
// LLVM: load atomic ptr, ptr @codegen..typeAssert.0 seq_cst
// LLVM: ptrtoint ptr
// LLVM: call goabiinternal ptr @"runtime.typeAssert<builtin.{{[0-9]+}}>"(ptr @codegen..typeAssert.0, ptr
// LLVM: !goobj.gotype = !{
// LLVM-DAG: !{ptr @codegen..typeAssert.0, ptr @

type llvmSourceInterface interface {
	Double() int
	Value(int) int
}

type llvmTargetInterface interface {
	Value(int) int
}

func convertLLVMInterface(v llvmSourceInterface) llvmTargetInterface {
	return v
}
