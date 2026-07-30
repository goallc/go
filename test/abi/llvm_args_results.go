// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file is also compiled directly by the LLVM ABI differential test. Keep
// every ABI case in this one source so the native and GoALLC objects cannot
// silently drift apart.
package main

import "runtime"

type stackAggregate struct {
	left, right int
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

	const depth = 2000
	if got, want := growPointer(&values[0], depth), values[0]*(depth+1); got != want {
		panic("pointer lost across stack growth or GC")
	}

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
}
