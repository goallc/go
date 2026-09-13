// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-LABEL: define goabiinternal {{.*}} @codegen.addressedPartiallyInitializedAggregateResult(
// LLVM-SAME: ptr goret(%codegen.pointerAggregate) align 8 "goretindex"="15" %.result15)
// The first field's redundant zero is removed, but every field must be
// initialized before the aggregate becomes observable at the call.
// LLVM: [[RAW_SECOND:%.*]] = getelementptr i8, ptr %.result15, i64 16
// LLVM: [[RAW_SCALAR:%.*]] = getelementptr i8, ptr %.result15, i64 8
// LLVM: [[RAW_FIRST:%.*]] = getelementptr i8, ptr %.result15, i64 0
// LLVM: store i64 0, ptr [[RAW_SCALAR]]
// LLVM: store ptr null, ptr [[RAW_SECOND]]
// LLVM: store ptr %first, ptr [[RAW_FIRST]]
// LLVM: call goabiinternal void @codegen.observePointerAggregate(ptr %.result15)
// LLVM: call goabiinternal void @codegen.safepoint()
// LLVM: store ptr %second, ptr [[RAW_SECOND]]
// LLVM-LABEL: define goabiinternal {{.*}} @codegen.partiallyInitializedAggregateResult(
// LLVM-SAME: ptr goret(%codegen.pointerAggregate) align 8 "goretindex"="15" %.result15)
// LLVM: call goabiinternal void @codegen.safepoint()
// LLVM: call goabiinternal void @codegen.safepoint()
// LLVM: store %codegen.pointerAggregate {{%.*}}, ptr %.result15
// LLVM-LABEL: define goabiinternal {{.*}} @codegen.deferredAggregateResult(
// LLVM-SAME: ptr goret(%codegen.pointerAggregate) align 8 "goallc.defer_result" "goretindex"="15" %.result15)
// LLVM-LABEL: define goabiinternal {{.*}} @codegen.initializedPointerResult(
// LLVM-SAME: ptr goret(ptr) align 8 "goretindex"="16" %.result16)
// LLVM: call goabiinternal void @codegen.safepoint()
// LLVM: store ptr %pointer, ptr %.result16
// LLVM-LABEL: define goabiinternal { ptr, ptr } @codegen.liveAggregateStackArgument(
// LLVM-SAME: ptr byval(%codegen.pointerAggregate) align 8 %value)
// LLVM: load %codegen.pointerAggregate, ptr %value
// LLVM: call goabiinternal void @codegen.safepoint()
// LLVM-LABEL: define goabiinternal ptr @codegen.liveScalarStackArgument(
// LLVM-SAME: ptr byval(ptr) align 8 %pointer)
// LLVM: load ptr, ptr %pointer
// LLVM: call goabiinternal void @codegen.safepoint()
// LLVM-OPT-LABEL: define goabiinternal {{.*}} @codegen.addressedPartiallyInitializedAggregateResult(
// LLVM-OPT-SAME: ptr goret(%codegen.pointerAggregate) align 8 {{.*}}"goretindex"="15" %.result15)
// LLVM-OPT: [[SECOND_RESULT:%.*]] = getelementptr {{.*}}ptr %.result15, i64 16
// LLVM-OPT: store ptr null, ptr [[SECOND_RESULT]]
// LLVM-OPT: store ptr %first, ptr %.result15
// LLVM-OPT: call goabiinternal void @codegen.observePointerAggregate(ptr {{.*}}%.result15)
// LLVM-OPT: call goabiinternal void @codegen.safepoint()
// LLVM-OPT: store ptr %second, ptr [[SECOND_RESULT]]
// LLVM-OPT-LABEL: define goabiinternal {{.*}} @codegen.deferredAggregateResult(
// LLVM-OPT-SAME: ptr goret(%codegen.pointerAggregate) align 8 "goallc.defer_result" "goretindex"="15" %.result15)

type pointerAggregate struct {
	first  *int
	scalar uintptr
	second *int
}

//go:noescape
func safepoint()

//go:noescape
func observePointerAggregate(*pointerAggregate)

//go:noinline
func initializedPointerResult(pointer *int) (
	r0, r1, r2, r3, r4, r5, r6, r7 int,
	r8, r9, r10, r11, r12, r13, r14, r15 int,
	result *int,
) {
	result = pointer
	safepoint()
	return
}

//go:noinline
func partiallyInitializedAggregateResult(first, second *int) (
	r0, r1, r2, r3, r4, r5, r6, r7 int,
	r8, r9, r10, r11, r12, r13, r14 int,
	result pointerAggregate,
) {
	result.first = first
	safepoint()
	result.second = second
	safepoint()
	return
}

//go:noinline
func addressedPartiallyInitializedAggregateResult(first, second *int) (
	r0, r1, r2, r3, r4, r5, r6, r7 int,
	r8, r9, r10, r11, r12, r13, r14 int,
	result pointerAggregate,
) {
	result.first = first
	observePointerAggregate(&result)
	safepoint()
	result.second = second
	return
}

// deferredAggregateResult uses a caller-owned result home that can be reached
// through defer recovery. Its pointer contents therefore remain visible to
// every suspension point, including paths that do not follow an ordinary call
// edge.
//
//go:noinline
func deferredAggregateResult(first, second *int) (
	r0, r1, r2, r3, r4, r5, r6, r7 int,
	r8, r9, r10, r11, r12, r13, r14 int,
	result pointerAggregate,
) {
	result = pointerAggregate{first: first, second: second}
	defer func() {
		result.first, result.second = result.second, result.first
	}()
	safepoint()
	return
}

// liveScalarStackArgument fills all sixteen arm64 integer argument registers.
// pointer is therefore loaded from a fixed incoming stack slot and remains live
// across safepoint.
//
//go:noinline
func liveScalarStackArgument(
	a0, a1, a2, a3, a4, a5, a6, a7 uintptr,
	a8, a9, a10, a11, a12, a13, a14, a15 uintptr,
	pointer *int,
) *int {
	safepoint()
	return pointer
}

// liveAggregateStackArgument leaves only two integer argument registers. The
// three-word aggregate cannot be split, so both of its pointer fields have
// exact fixed incoming stack homes at the safepoint.
//
//go:noinline
func liveAggregateStackArgument(
	a0, a1, a2, a3, a4, a5, a6 uintptr,
	a7, a8, a9, a10, a11, a12, a13 uintptr,
	value pointerAggregate,
) (*int, *int) {
	safepoint()
	return value.first, value.second
}
