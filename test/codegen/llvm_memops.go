// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-DAG: define goabiinternal void @codegen.llvmZeroByte(
// LLVM-DAG: store i8 0, ptr %dst, align 1
func llvmZeroByte(dst *[1]byte) {
	*dst = [1]byte{}
}

// LLVM-DAG: define goabiinternal void @codegen.llvmZeroBytes(
// LLVM-DAG: call void @llvm.memset.p0.i64(ptr align 1 %dst, i8 0, i64 24, i1 false)
func llvmZeroBytes(dst *[24]byte) {
	*dst = [24]byte{}
}

// LLVM-DAG: define goabiinternal void @codegen.llvmZeroAligned(
// LLVM-DAG: call void @llvm.memset.p0.i64(ptr align 8 %dst, i8 0, i64 24, i1 false)
func llvmZeroAligned(dst *[3]uint64) {
	*dst = [3]uint64{}
}

type llvmPointerStackZero [4]*int

type llvmPointerStackContainer struct {
	pad    uintptr
	values llvmPointerStackZero
}

//go:noescape
func llvmPointerStackZeroSink(*llvmPointerStackZero)

// LLVM-DAG: define goabiinternal ptr @codegen.llvmZeroFreshPointerStack(
// LLVM-DAG: {{%.*}} = alloca [4 x ptr], align 8
// LLVM-DAG: call void @llvm.memset.p0.i64(ptr align 8 {{%.*}}, i8 0, i64 32, i1 false)
func llvmZeroFreshPointerStack(p *int) *int {
	var local llvmPointerStackZero
	local[3] = p
	llvmPointerStackZeroSink(&local)
	return local[3]
}

// LLVM-DAG: define goabiinternal void @codegen.llvmZeroReusedPointerStack(
// LLVM-DAG: call void @llvm.memset.p0.i64(ptr align 8 {{%.*}}, i8 0, i64 32, i1 false)
func llvmZeroReusedPointerStack(p *int) {
	var local llvmPointerStackZero
	local[0] = p
	llvmPointerStackZeroSink(&local)
	local = llvmPointerStackZero{}
	llvmPointerStackZeroSink(&local)
}

// LLVM-DAG: define goabiinternal void @codegen.llvmZeroDerivedPointerStack(
// LLVM-DAG: getelementptr i8, ptr {{%.*}}, i64 8
// LLVM-DAG: call void @llvm.memset.p0.i64(ptr align 8 {{%.*}}, i8 0, i64 32, i1 false)
func llvmZeroDerivedPointerStack(p *int) {
	var local llvmPointerStackContainer
	local.values[0] = p
	llvmPointerStackZeroSink(&local.values)
	local.values = llvmPointerStackZero{}
	llvmPointerStackZeroSink(&local.values)
}

// LLVM-DAG: define goabiinternal ptr @codegen.llvmMovePointerToStack(
// LLVM-DAG: call void @llvm.memmove.p0.p0.i64(ptr align 8 {{%.*}}, ptr align 8 %src, i64 32, i1 false)
func llvmMovePointerToStack(src *llvmPointerStackZero) *int {
	var local llvmPointerStackZero
	local = *src
	llvmPointerStackZeroSink(&local)
	return local[3]
}

// LLVM-DAG: define goabiinternal void @codegen.llvmMoveOverlapSized(
// LLVM-DAG: call void @llvm.memmove.p0.p0.i64(ptr align 1 %dst, ptr align 1 %src, i64 32, i1 false)
func llvmMoveOverlapSized(dst, src *[32]byte) {
	*dst = *src
}

// LLVM-DAG: define goabiinternal void @codegen.llvmMoveAligned(
// LLVM-ARM64-DAG: call void @llvm.memmove.p0.p0.i64(ptr align 8 %dst, ptr align 8 %src, i64 24, i1 false)
// LLVM-AMD64-DAG: call void @llvm.memmove.p0.p0.i64(ptr align 1 %dst, ptr align 1 %src, i64 24, i1 false)
func llvmMoveAligned(dst, src *[3]uint64) {
	*dst = *src
}

// LLVM-DAG: define goabiinternal void @codegen.llvmMoveLarge(
// LLVM-DAG: call void @llvm.memmove.p0.p0.i64(ptr align 1 %dst, ptr align 1 {{%.*}}, i64 128, i1 false)
func llvmMoveLarge(dst *[128]byte, src [128]byte) {
	*dst = src
}

// LLVM-DAG: define goabiinternal i1 @codegen.llvmMemEq(
// LLVM-DAG: call goabiinternal i1 @"runtime.memequal<builtin.{{[0-9]+}}>"(ptr {{%.*}}, ptr {{%.*}}, i64 {{%.*}}){{$|,}}
func llvmMemEq(a, b string) bool {
	return a == b
}

// LLVM-DAG: define goabiinternal { ptr, i64, i64 } @codegen.llvmSlicemask(
// LLVM-DAG: {{%.*}} = sub i64 0, {{%.*}}
// LLVM-DAG: {{%.*}} = ashr i64 {{%.*}}, 63
func llvmSlicemask(a []byte, i int) []byte {
	return a[i:]
}

// LLVM-DAG: define goabiinternal i1 @codegen.llvmStringLess(
// LLVM-DAG: call goabiinternal i64 @"runtime.cmpstring<builtin.{{[0-9]+}}>"(
func llvmStringLess(a, b string) bool {
	return a < b
}

// GC-leaf contracts belong to the runtime declarations, not individual calls.
// LLVM-DAG: declare goabiinternal i1 @"runtime.memequal<builtin.{{[0-9]+}}>"(ptr readonly captures(none), ptr readonly captures(none), i64) #[[COMPARE:[0-9]+]]
// LLVM-DAG: declare goabiinternal void @"runtime.memmove<builtin.{{[0-9]+}}>"(ptr writeonly captures(none), ptr readonly captures(none), i64) #[[RAW_MEMORY:[0-9]+]]
// LLVM-DAG: declare goabiinternal range(i64 -1, 2) i64 @"runtime.cmpstring<builtin.{{[0-9]+}}>"({ ptr, i64 }, { ptr, i64 }) #[[COMPARE]]
// LLVM-DAG: attributes #[[RAW_MEMORY]] = { nocallback nofree nounwind "gc-leaf-function" }

// LLVM-DAG: attributes #[[COMPARE]] = { nocallback nofree nosync nounwind willreturn memory(read) "gc-leaf-function" }

// LLVM-DAG: define goabiinternal void @codegen.llvmMoveDynamic(
// LLVM-DAG: call void @llvm.memmove.p0.p0.i64(ptr align 1 {{%.*}}, ptr align 1 {{%.*}}, i64 {{%.*}}, i1 false)
func llvmMoveDynamic(dst, src []byte) {
	copy(dst, src)
}

// LLVM-DAG: define goabiinternal void @codegen.llvmClearDynamic(
// LLVM-DAG: call void @llvm.memset.p0.i64(ptr align 1 {{%.*}}, i8 0, i64 {{%.*}}, i1 false)
func llvmClearDynamic(dst []byte) {
	clear(dst)
}

// LLVM-DAG: define goabiinternal void @codegen.llvmMoveDeferResult(
// LLVM-DAG: call void @llvm.memmove.p0.p0.i64({{.*}}i64 128, i1 true)
func llvmMoveDeferResult(src *[128]byte) (r [128]byte) {
	defer func() { r[0]++ }()
	r = *src
	return
}
