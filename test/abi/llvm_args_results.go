// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file is also compiled directly by the LLVM ABI differential test. Keep
// every ABI case in this one source so the native and GoALLC objects cannot
// silently drift apart.
package main

import (
	"runtime"
	"unsafe"
)

type stackAggregate struct {
	left, right int
}

type pointerStackAggregate struct {
	first  *int
	scalar int
	second *int
}

var values = [8]int{11, 13, 17, 19, 23, 29, 31, 37}

//go:noinline
func checkpoint(pointer *int) {
	runtime.GC()
	if *pointer == 0 {
		panic("checkpoint pointer lost")
	}
}

// mixedABI fills all sixteen arm64 integer parameter registers, then passes an
// intrinsically stack-assigned aggregate and one budget-overflow pointer.
//
//go:noinline
func mixedABI(
	a0 int, p0 *int,
	a1, a2, a3, a4, a5, a6, a7 int,
	a8, a9, a10, a11, a12, a13 int,
	aggregate stackAggregate, tail, overflow *int,
) int {
	checkpoint(p0)
	return a0 + a1 + a2 + a3 + a4 + a5 + a6 + a7 +
		a8 + a9 + a10 + a11 + a12 + a13 +
		*p0 + aggregate.left + aggregate.right + *tail + *overflow
}

// liveScalarStackArgument fills all sixteen arm64 integer argument registers,
// leaving pointer in the first incoming stack slot while runtime.GC executes.
//
//go:noinline
func liveScalarStackArgument(
	a0, a1, a2, a3, a4, a5, a6, a7 int,
	a8, a9, a10, a11, a12, a13, a14, a15 int,
	pointer *int,
) (*int, int) {
	runtime.GC()
	return pointer, a0 + a1 + a2 + a3 + a4 + a5 + a6 + a7 +
		a8 + a9 + a10 + a11 + a12 + a13 + a14 + a15
}

// livePointerSequenceStackArguments places two pointer stack arguments around
// a non-pointer word and keeps both pointers live across runtime.GC.
//
//go:noinline
func livePointerSequenceStackArguments(
	a0, a1, a2, a3, a4, a5, a6, a7 int,
	a8, a9, a10, a11, a12, a13, a14, a15 int,
	first *int, scalar int, second *int,
) (*int, *int, int) {
	runtime.GC()
	return first, second, a0 + a15 + scalar
}

// livePointerAggregateStackArgument leaves only two arm64 integer argument
// registers. The three-word aggregate cannot be split and therefore has two
// independently tracked pointer leaves in fixed incoming stack slots.
//
//go:noinline
func livePointerAggregateStackArgument(
	a0, a1, a2, a3, a4, a5, a6 int,
	a7, a8, a9, a10, a11, a12, a13 int,
	value pointerStackAggregate,
) (*int, *int, int) {
	runtime.GC()
	return value.first, value.second, a0 + a13 + value.scalar
}

// growPointer keeps an entry pointer live through both recursive stack growth
// and a runtime GC.
//
//go:noinline
func growPointer(pointer *int, depth int) int {
	if depth == 0 {
		checkpoint(pointer)
		return *pointer
	}
	return growPointer(pointer, depth-1) + *pointer
}

//go:noinline
func stackAddress(pointer *byte) uintptr {
	return uintptr(unsafe.Pointer(pointer))
}

// stackResultsAfterGrowth keeps caller-owned result slots live while a nested
// call grows the Go stack. A caller must reload its current stack pointer after
// this call; retaining the pre-call SP would read results from the retired
// stack segment.
//
//go:noinline
func stackResultsAfterGrowth(pointer *int) (
	r0, r1, r2, r3, r4, r5, r6, r7 int,
	r8, r9, r10, r11, r12, r13, r14, r15 int,
	result *int,
) {
	var marker byte
	oldStackAddress := stackAddress(&marker)
	checksum := growPointer(pointer, 2000)
	if stackAddress(&marker) == oldStackAddress {
		panic("stackResultsAfterGrowth did not grow the stack")
	}
	return 0, 1, 2, 3, 4, 5, 6, 7,
		8, 9, 10, 11, 12, 13, 14, checksum, pointer
}

// overflowResults uses all sixteen arm64 integer result registers and two
// caller-owned result slots. Pointer results occupy both register and stack
// positions.
//
//go:noinline
func overflowResults(p0, p15, p16, p17 *int) (
	*int,
	int, int, int, int, int, int, int,
	int, int, int, int, int, int, int,
	*int, *int, *int,
) {
	checkpoint(p0)
	return p0,
		1, 2, 3, 4, 5, 6, 7,
		8, 9, 10, 11, 12, 13, 14,
		p15, p16, p17
}

// initializedStackResult uses all sixteen arm64 integer result registers before
// a caller-owned pointer result slot. Its source assignment precedes the
// safepoint, but either backend may defer the physical result-slot store. The
// differential test therefore checks the emitted maps rather than assuming
// that this slot is already a root.
//
//go:noinline
func initializedStackResult(pointer *int) (
	r0, r1, r2, r3, r4, r5, r6, r7 int,
	r8, r9, r10, r11, r12, r13, r14, r15 int,
	result *int,
) {
	r0, r1, r2, r3, r4, r5, r6, r7 = 0, 1, 2, 3, 4, 5, 6, 7
	r8, r9, r10, r11, r12, r13, r14, r15 = 8, 9, 10, 11, 12, 13, 14, 15
	result = pointer
	runtime.GC()
	return
}

// stackAggregateResult places fifteen scalar results first, so the following
// two-word aggregate cannot fit in the one remaining result register. The
// final pointer still uses that remaining register.
//
//go:noinline
func stackAggregateResult(left, right int, pointer *int) (
	int, int, int, int, int, int, int, int,
	int, int, int, int, int, int, int,
	stackAggregate, *int,
) {
	checkpoint(pointer)
	return 1, 2, 3, 4, 5, 6, 7, 8,
		9, 10, 11, 12, 13, 14, 15,
		stackAggregate{left: left, right: right}, pointer
}

// bothOverflow combines the mixed input layout with the same overflowing
// result layout as overflowResults. This catches layouts that accidentally
// place register-argument homes before caller-owned stack results.
//
//go:noinline
func bothOverflow(
	a0 int, p0 *int,
	a1, a2, a3, a4, a5, a6, a7 int,
	a8, a9, a10, a11, a12, a13 int,
	aggregate stackAggregate, tail, overflow *int,
) (
	*int,
	int, int, int, int, int, int, int,
	int, int, int, int, int, int, int,
	*int, *int, *int,
) {
	checkpoint(p0)
	return p0,
		a0, a1, a2, a3, a4, a5, a6,
		a7, a8, a9, a10, a11, a12, a13,
		&values[1+aggregate.left-13], tail, overflow
}

// pointerAggregateBothOverflow combines a pointer-containing aggregate that is
// entirely stack-assigned with two pointer results after all sixteen integer
// result registers. The input pointer leaves are live across runtime.GC and the
// returned pointers occupy caller-owned result slots.
//
//go:noinline
func pointerAggregateBothOverflow(
	a0, a1, a2, a3, a4, a5, a6 int,
	a7, a8, a9, a10, a11, a12, a13 int,
	value pointerStackAggregate,
) (
	int, int, int, int, int, int, int, int,
	int, int, int, int, int, int, int, int,
	*int, *int,
) {
	runtime.GC()
	return a0, a1, a2, a3, a4, a5, a6, a7,
		a8, a9, a10, a11, a12, a13, value.scalar, a0 + a13,
		value.first, value.second
}

func requirePointer(got, want *int) {
	if got != want || *got != *want {
		panic("ABI pointer result mismatch")
	}
}

//go:noinline
func requireAggregate(got stackAggregate, left, right int, pointer *int) {
	runtime.GC()
	requirePointer(pointer, &values[0])
	if got != (stackAggregate{left: left, right: right}) {
		panic("ABI aggregate payload mismatch")
	}
}

func main() {
	got := mixedABI(
		1, &values[0],
		2, 3, 4, 5, 6, 7, 8,
		9, 10, 11, 12, 13, 14,
		stackAggregate{left: 13, right: 17}, &values[4], &values[3],
	)
	const scalarSum = 105
	if want := scalarSum + values[0] + 13 + 17 + values[4] + values[3]; got != want {
		panic("mixed register and stack arguments mismatch")
	}

	scalarPointer, scalarChecksum := liveScalarStackArgument(
		1, 2, 3, 4, 5, 6, 7, 8,
		9, 10, 11, 12, 13, 14, 15, 16,
		&values[2],
	)
	runtime.GC()
	requirePointer(scalarPointer, &values[2])
	if scalarChecksum != 136 {
		panic("scalar stack argument checksum mismatch")
	}

	sequenceFirst, sequenceSecond, sequenceChecksum := livePointerSequenceStackArguments(
		1, 2, 3, 4, 5, 6, 7, 8,
		9, 10, 11, 12, 13, 14, 15, 16,
		&values[1], 41, &values[6],
	)
	runtime.GC()
	requirePointer(sequenceFirst, &values[1])
	requirePointer(sequenceSecond, &values[6])
	if sequenceChecksum != 58 {
		panic("pointer sequence stack argument checksum mismatch")
	}

	aggregateFirst, aggregateSecond, aggregateChecksum := livePointerAggregateStackArgument(
		1, 2, 3, 4, 5, 6, 7,
		8, 9, 10, 11, 12, 13, 14,
		pointerStackAggregate{first: &values[2], scalar: 43, second: &values[7]},
	)
	runtime.GC()
	requirePointer(aggregateFirst, &values[2])
	requirePointer(aggregateSecond, &values[7])
	if aggregateChecksum != 58 {
		panic("pointer aggregate stack argument checksum mismatch")
	}

	const depth = 2000
	if got, want := growPointer(&values[0], depth), values[0]*(depth+1); got != want {
		panic("pointer lost across stack growth or GC")
	}

	// Use a fresh goroutine so stackResultsAfterGrowth necessarily starts on a
	// small stack and grows it while its caller-owned result area is active.
	done := make(chan struct{})
	go func() {
		_, _, _, _, _, _, _, _,
			_, _, _, _, _, _, _, checksum,
			pointer := stackResultsAfterGrowth(&values[1])
		if want := values[1] * 2001; checksum != want {
			panic("stack result lost across stack growth")
		}
		runtime.GC()
		requirePointer(pointer, &values[1])
		close(done)
	}()
	<-done

	r0,
		_, _, _, _, _, _, _,
		_, _, _, _, _, _, _,
		r15, r16, r17 := overflowResults(&values[0], &values[5], &values[6], &values[7])
	runtime.GC()
	requirePointer(r0, &values[0])
	requirePointer(r15, &values[5])
	requirePointer(r16, &values[6])
	requirePointer(r17, &values[7])

	_, _, _, _, _, _, _, _,
		_, _, _, _, _, _, _, _,
		initialized := initializedStackResult(&values[2])
	runtime.GC()
	requirePointer(initialized, &values[2])

	_, _, _, _, _, _, _, _,
		_, _, _, _, _, _, _,
		resultAggregate, resultPointer := stackAggregateResult(19, 23, &values[0])
	requireAggregate(resultAggregate, 19, 23, resultPointer)

	b0,
		_, _, _, _, _, _, _,
		_, _, _, _, _, _, _,
		b15, b16, b17 := bothOverflow(
		1, &values[0],
		2, 3, 4, 5, 6, 7, 8,
		9, 10, 11, 12, 13, 14,
		stackAggregate{left: 13, right: 17}, &values[4], &values[3],
	)
	runtime.GC()
	requirePointer(b0, &values[0])
	requirePointer(b15, &values[1])
	requirePointer(b16, &values[4])
	requirePointer(b17, &values[3])

	_, _, _, _, _, _, _, _,
		_, _, _, _, _, _, _, _,
		bothAggregateFirst, bothAggregateSecond := pointerAggregateBothOverflow(
		1, 2, 3, 4, 5, 6, 7,
		8, 9, 10, 11, 12, 13, 14,
		pointerStackAggregate{first: &values[3], scalar: 47, second: &values[5]},
	)
	runtime.GC()
	requirePointer(bothAggregateFirst, &values[3])
	requirePointer(bothAggregateSecond, &values[5])
}
