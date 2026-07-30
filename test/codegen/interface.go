// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-DAG: %go.runtime.ITab = type <{ ptr, ptr, i32, [4 x i8], [1 x ptr] }>
// LLVM-DAG: @"go:itab.codegen.llvmInterfaceValue,codegen.llvmInterface" = weak constant <{ %go.runtime.ITab, [1 x ptr] }> {{.*}}!goobj.symbol.flags ![[ITAB_FLAGS:[0-9]+]]
// LLVM-DAG: ptr @"codegen.(*llvmInterfaceValue).Double"
// LLVM-DAG: ptr @"codegen.(*llvmInterfaceValue).Value"
// LLVM-DAG: !goobj.relocs
// LLVM-DAG: !goobj.weak_relocs
// LLVM-DAG: @llvm.compiler.used = appending global
// LLVM-LABEL: define goabiinternal i64 @codegen.useLLVMInterface(
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmInterface.Value(
// LLVM: load ptr, ptr
// LLVM: call goabiinternal i64 %
// LLVM-SAME: (ptr
// LLVM-SAME: i64
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmInterface.Double(
// LLVM: load ptr, ptr
// LLVM: call goabiinternal i64 %
// LLVM-SAME: (ptr
// LLVM: !goobj.marker_relocs = !{
// LLVM-DAG: !{ptr @codegen.useLLVMInterface, ptr @"type:codegen.llvmInterfaceValue", i32 23, i64 0}
// LLVM-DAG: !{ptr @codegen.llvmInterface.Value, ptr @"type:codegen.llvmInterface", i32 24, i64 {{[0-9]+}}}
// LLVM-DAG: !{ptr @codegen.llvmInterface.Double, ptr @"type:codegen.llvmInterface", i32 24, i64 {{[0-9]+}}}
// LLVM-DAG: ![[ITAB_FLAGS]] = !{i32 0, i32 2}
// LLVM-DAG: !{i32 {{[0-9]+}}, i32 5}

type llvmInterface interface {
	Double() int
	Value(int) int
}

type llvmInterfaceValue struct {
	value int
}

//go:noinline
func (v llvmInterfaceValue) Value(delta int) int {
	return v.value + delta
}

//go:noinline
func (v llvmInterfaceValue) Double() int {
	return v.value * 2
}

func useLLVMInterface(v llvmInterfaceValue) int {
	var i llvmInterface = v
	return i.Value(2) + i.Double()
}
