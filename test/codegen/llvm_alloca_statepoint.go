// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-LABEL: define goabiinternal i64 @codegen.localAcrossSafepoints(
// LLVM: alloca %codegen.pointerLocal, align 8
// LLVM: call void @llvm.lifetime.start.p0(ptr
// LLVM: call void @llvm.memset.inline.p0.i64(ptr align 8 {{%.*}}, i8 0, i64 40
// LLVM: call goabiinternal void @codegen.mutateLocal(
// LLVM: call goabiinternal void @codegen.safepoint()
// LLVM-LABEL: define goabiinternal void @codegen.stackParameterAcrossSafepoints(
// LLVM-SAME: ptr byval([2 x ptr]) align 8 %value)
// LLVM-NOT: alloca [2 x ptr]
// LLVM: call goabiinternal void @codegen.mutatePointerArray(ptr %value)
// LLVM: call goabiinternal void @codegen.safepoint()
// LLVM: call goabiinternal void @codegen.mutatePointerArray(ptr %value)
// LLVM-LABEL: define goabiinternal void @codegen.parameterAcrossSafepoints(
// LLVM-SAME: ptr byval(%codegen.pointerLocal) align 8 %value)
// LLVM-NOT: alloca %codegen.pointerLocal
// LLVM: call goabiinternal void @codegen.mutateLocal(ptr %value, i8 0)
// LLVM: call goabiinternal void @codegen.safepoint()
// LLVM: call goabiinternal void @codegen.mutateLocal(ptr %value, i8 1)

type pointerLocal struct {
	first  *int
	scalar uintptr
	second *int
	tail   [2]*int
}

type pointerArray [2]*int

var (
	globalFirst  *int
	globalSecond *int
	globalThird  *int
	globalFourth *int
)

//go:noescape
func safepoint()

// mutateLocal is intentionally opaque to both compilers. A real callee may
// update the pointer fields while the call is a safepoint, so GoALLC must scan
// the original alloca memory rather than write back pre-call SSA values.
//
//go:noescape
func mutateLocal(value *pointerLocal, branch bool)

//go:noescape
func mutatePointerArray(value *pointerArray)

// localAcrossSafepoints deliberately takes the address of a pointer-containing
// local. Its pointer leaves must remain rooted in that original stack object
// across both safepoint and mutateLocal. The latter may update the object, so a
// relocated pre-call value must not be stored over its write.
//
//go:noinline
func localAcrossSafepoints(branch bool, rounds int) uintptr {
	var value pointerLocal
	value.first = globalFirst
	value.scalar = 41
	value.second = globalSecond
	value.tail[0] = globalThird
	value.tail[1] = globalFourth

	mutateLocal(&value, branch)
	safepoint()
	for i := 0; i < rounds; i++ {
		if i&1 == 0 {
			safepoint()
		} else {
			mutateLocal(&value, !branch)
		}
	}
	return value.scalar
}

// parameterAcrossSafepoints takes the address of an ABIInternal aggregate
// parameter. SelectionDAG must assign its canonical alloca to the real
// argument home. The final call does not keep the direct alloca base live-out,
// so the object also exercises the function-level argp-relative StackObject.
//
//go:noinline
func parameterAcrossSafepoints(value pointerLocal) {
	mutateLocal(&value, false)
	safepoint()
	mutateLocal(&value, true)
}

// stackParameterAcrossSafepoints exercises the same address identity for an
// aggregate assigned wholly to the ABI stack. Its canonical alloca must reuse
// the caller-populated incoming slot rather than copying into a local object.
//
//go:noinline
func stackParameterAcrossSafepoints(value pointerArray) {
	mutatePointerArray(&value)
	safepoint()
	mutatePointerArray(&value)
}
