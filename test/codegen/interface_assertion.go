// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

type llvmAssertionInterface interface {
	Value(int) int
}

type llvmAssertionValue int

//go:noinline
func (v llvmAssertionValue) Value(delta int) int {
	return int(v) + delta
}

// LLVM: @codegen..typeAssert.0 = internal global <{ ptr, ptr, [8 x i8] }>
// LLVM-LABEL: define goabiinternal { { ptr, ptr }, i8 } @codegen.assertLLVMInterface(
// LLVM: load atomic ptr, ptr @codegen..typeAssert.0 seq_cst
// LLVM: call goabiinternal ptr @"runtime.typeAssert<builtin.{{[0-9]+}}>"(ptr @codegen..typeAssert.0, ptr
// LLVM-LABEL: define goabiinternal { i64, i8 } @codegen.assertLLVMConcrete(
// LLVM: extractvalue { ptr, ptr }
// LLVM: icmp eq ptr
// LLVM-SAME: @"type:codegen.llvmAssertionValue"
// LLVM: load i64, ptr
func assertLLVMConcrete(v any) (llvmAssertionValue, bool) {
	x, ok := v.(llvmAssertionValue)
	return x, ok
}

func assertLLVMInterface(v any) (llvmAssertionInterface, bool) {
	x, ok := v.(llvmAssertionInterface)
	return x, ok
}

// LLVM-DAG: call goabiinternal void @"runtime.panicdottypeE<builtin.{{[0-9]+}}>"(
// LLVM-DAG: call goabiinternal void @"runtime.panicnildottype<builtin.{{[0-9]+}}>"(
func mustLLVMConcrete(v any) llvmAssertionValue {
	return v.(llvmAssertionValue)
}

func mustLLVMInterface(v any) llvmAssertionInterface {
	return v.(llvmAssertionInterface)
}

// LLVM: !goobj.gotype = !{
// LLVM-DAG: !{ptr @codegen..typeAssert.0, ptr @
