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
// LLVM: call void @llvm.sideeffect(), !goobj.marker_reloc ![[USE_IFACE:[0-9]+]]
// LLVM-LABEL: define weak goabiinternal i64 @codegen.llvmInterface.Value(
// LLVM: call void @llvm.sideeffect(), !goobj.marker_reloc ![[VALUE_METHOD:[0-9]+]]
// LLVM: load ptr, ptr
// LLVM: call goabiinternal i64 %
// LLVM-SAME: (ptr
// LLVM-SAME: i64
// LLVM-LABEL: define weak goabiinternal i64 @codegen.llvmInterface.Double(
// LLVM: call void @llvm.sideeffect(), !goobj.marker_reloc ![[DOUBLE_METHOD:[0-9]+]]
// LLVM: load ptr, ptr
// LLVM: call goabiinternal i64 %
// LLVM-SAME: (ptr
// LLVM-DAG: define goabiinternal { i64, ptr } @codegen.makeLLVMDirectIfaceStruct(
// LLVM-DAG: extractvalue %codegen.llvmDirectIfaceStruct %{{.*}}, 0
// LLVM-DAG: insertvalue { i64, ptr } { i64 ptrtoint (ptr @"type:codegen.llvmDirectIfaceStruct" to i64), ptr undef }, ptr %{{.*}}, 1
// LLVM-DAG: define goabiinternal { i64, ptr } @codegen.makeLLVMDirectIfaceArray(
// LLVM-DAG: extractvalue [1 x ptr] %{{.*}}, 0
// LLVM-DAG: insertvalue { i64, ptr } { i64 ptrtoint (ptr @"type:codegen.llvmDirectIfaceArray" to i64), ptr undef }, ptr %{{.*}}, 1
// LLVM-DAG: define goabiinternal ptr @codegen.unboxLLVMDirectIfaceStruct(
// LLVM-DAG: extractvalue { i64, ptr } %{{.*}}, 1
// LLVM-DAG: define goabiinternal ptr @codegen.unboxLLVMDirectIfaceArray(
// LLVM-DAG: extractvalue { i64, ptr } %{{.*}}, 1
// LLVM-DAG: ![[USE_IFACE]] = !{ptr @"type:codegen.llvmInterfaceValue", i32 23, i64 0}
// LLVM-DAG: ![[VALUE_METHOD]] = !{ptr @"type:codegen.llvmInterface", i32 24, i64 {{[0-9]+}}}
// LLVM-DAG: ![[DOUBLE_METHOD]] = !{ptr @"type:codegen.llvmInterface", i32 24, i64 {{[0-9]+}}}
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

type llvmDirectIfaceStruct struct {
	p *int
}

type llvmDirectIfaceArray [1]*int

func makeLLVMDirectIfaceArray(v llvmDirectIfaceArray) any {
	return v
}

func makeLLVMDirectIfaceStruct(v llvmDirectIfaceStruct) any {
	return v
}

func unboxLLVMDirectIfaceArray(v any) *int {
	return v.(llvmDirectIfaceArray)[0]
}

func unboxLLVMDirectIfaceStruct(v any) *int {
	return v.(llvmDirectIfaceStruct).p
}
