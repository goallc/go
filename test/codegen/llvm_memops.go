// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-DAG: define goabiinternal void @codegen.llvmZeroByte(
// LLVM-DAG: store [1 x i8] zeroinitializer, ptr %dst, align 1
func llvmZeroByte(dst *[1]byte) {
	*dst = [1]byte{}
}

// LLVM-DAG: define goabiinternal void @codegen.llvmZeroBytes(
// LLVM-DAG: call void @llvm.memset.inline.p0.i64(ptr align 1 %dst, i8 0, i64 24, i1 false)
func llvmZeroBytes(dst *[24]byte) {
	*dst = [24]byte{}
}

// LLVM-DAG: define goabiinternal void @codegen.llvmZeroAligned(
// LLVM-DAG: call void @llvm.memset.inline.p0.i64(ptr align 8 %dst, i8 0, i64 24, i1 false)
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
// LLVM-DAG: call void @llvm.memset.inline.p0.i64(ptr align 8 {{%.*}}, i8 0, i64 32, i1 false)
func llvmZeroFreshPointerStack(p *int) *int {
	var local llvmPointerStackZero
	local[3] = p
	llvmPointerStackZeroSink(&local)
	return local[3]
}

// LLVM-DAG: define goabiinternal void @codegen.llvmZeroReusedPointerStack(
// LLVM-DAG: call void @llvm.memset.inline.p0.i64(ptr align 8 {{%.*}}, i8 0, i64 32, i1 false)
func llvmZeroReusedPointerStack(p *int) {
	var local llvmPointerStackZero
	local[0] = p
	llvmPointerStackZeroSink(&local)
	local = llvmPointerStackZero{}
	llvmPointerStackZeroSink(&local)
}

// LLVM-DAG: define goabiinternal void @codegen.llvmZeroDerivedPointerStack(
// LLVM-DAG: getelementptr i8, ptr {{%.*}}, i64 8
// LLVM-DAG: call void @llvm.memset.inline.p0.i64(ptr align 8 {{%.*}}, i8 0, i64 32, i1 false)
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
// LLVM-ARM64-DAG: call void @llvm.memmove.p0.p0.i64(ptr align 1 %dst, ptr align 1 %src, i64 32, i1 false)
// LLVM-AMD64-DAG: call goabiinternal void @runtime.memmove(ptr %dst, ptr %src, i64 32) #{{[0-9]+}}
func llvmMoveOverlapSized(dst, src *[32]byte) {
	*dst = *src
}

// LLVM-DAG: define goabiinternal void @codegen.llvmMoveAligned(
// LLVM-ARM64-DAG: call void @llvm.memmove.p0.p0.i64(ptr align 8 %dst, ptr align 8 %src, i64 24, i1 false)
// LLVM-AMD64-DAG: call goabiinternal void @runtime.memmove(ptr %dst, ptr %src, i64 24) #{{[0-9]+}}
func llvmMoveAligned(dst, src *[3]uint64) {
	*dst = *src
}

// LLVM-DAG: define goabiinternal void @codegen.llvmMoveLarge(
// LLVM-DAG: call goabiinternal void @runtime.memmove(ptr %dst, ptr {{%.*}}, i64 128) #{{[0-9]+}}
// LLVM-DAG: declare !goobj.builtin !{{[0-9]+}} goabiinternal void @runtime.memmove(ptr, ptr, i64) #{{[0-9]+}}
func llvmMoveLarge(dst *[128]byte, src [128]byte) {
	*dst = src
}

// LLVM-DAG: define goabiinternal i8 @codegen.llvmMemEq(
// LLVM-DAG: call goabiinternal i8 @runtime.memequal(ptr {{%.*}}, ptr {{%.*}}, i64 {{%.*}}) #{{[0-9]+}}
// LLVM-DAG: declare !goobj.builtin !{{[0-9]+}} goabiinternal i8 @runtime.memequal(ptr, ptr, i64) #{{[0-9]+}}
func llvmMemEq(a, b string) bool {
	return a == b
}

// LLVM-DAG: define goabiinternal { ptr, i64, i64 } @codegen.llvmSlicemask(
// LLVM-DAG: {{%.*}} = sub i64 0, {{%.*}}
// LLVM-DAG: {{%.*}} = ashr i64 {{%.*}}, 63
func llvmSlicemask(a []byte, i int) []byte {
	return a[i:]
}

// LLVM-DAG: attributes #{{[0-9]+}} = { "gc-leaf-function" }
