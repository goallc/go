// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

type pointerLocal struct {
	first  *int
	scalar int
	second *int
	tail   [2]*int
}

type pointerPair struct {
	first  *int
	second *int
}

//go:noinline
func newValue(value int) *int {
	pointer := new(int)
	*pointer = value
	return pointer
}

//go:noinline
func mutateLocal(value *pointerLocal, first, second *int, branch bool) {
	if branch {
		oldSecond := value.second
		value.first = first
		value.second = second
		value.tail[0] = oldSecond
		value.tail[1] = first
	} else {
		oldFirst := value.first
		value.first = second
		value.second = first
		value.tail[0] = second
		value.tail[1] = oldFirst
	}
}

//go:noinline
func movePair(destination *pointerPair, source pointerPair) {
	*destination = source
}

//go:noinline
func zeroPair(destination *pointerPair) {
	*destination = pointerPair{}
}

// grow keeps the caller's address-taken local live while recursion repeatedly
// grows and copies the stack. The pointer-free padding makes stack growth
// deterministic without adding unrelated pointer-map bits.
//
//go:noinline
func grow(value *pointerLocal, depth int) int {
	var padding [64]uintptr
	padding[0] = uintptr(depth)
	if depth == 0 {
		runtime.GC()
		return localSum(value)
	}
	return grow(value, depth-1) + int(padding[0]&1)
}

//go:noinline
func localSum(value *pointerLocal) int {
	return *value.first + value.scalar + *value.second +
		*value.tail[0] + *value.tail[1]
}

// exercise covers branch joins, repeated ordinary safepoints, callee writes to
// the original alloca, runtime stack growth, and a final GC before all pointer
// fields are read again.
//
//go:noinline
func exercise(branch bool) int {
	first := newValue(11)
	second := newValue(13)
	replacementFirst := newValue(17)
	replacementSecond := newValue(19)
	var value pointerLocal
	value.first = first
	value.scalar = 23
	value.second = second
	value.tail[0] = replacementFirst
	value.tail[1] = replacementSecond

	mutateLocal(&value, replacementFirst, replacementSecond, branch)
	first, second, replacementFirst, replacementSecond = nil, nil, nil, nil

	for i := 0; i < 4; i++ {
		runtime.GC()
		if i == 1 {
			mutateLocal(&value, value.tail[1], value.tail[0], !branch)
		}
	}

	const depth = 1200
	got := grow(&value, depth)
	runtime.GC()
	return got + localSum(&value)
}

// exerciseWriteBarrierMutation covers the aggregate replacement and clearing
// semantics used by wbMove/wbZero lowering in callees that receive the address
// of the caller's pointer-containing local. A statepoint must not restore the
// pre-call contents over either mutation.
//
//go:noinline
func exerciseWriteBarrierMutation() {
	first := newValue(29)
	second := newValue(31)
	var value pointerPair
	movePair(&value, pointerPair{first: first, second: second})
	first, second = nil, nil
	runtime.GC()
	if value.first == nil || value.second == nil ||
		*value.first != 29 || *value.second != 31 {
		panic("wbMove mutation of address-taken local was lost")
	}
	zeroPair(&value)
	runtime.GC()
	if value.first != nil || value.second != nil {
		panic("wbZero mutation of address-taken local was overwritten")
	}
}

func main() {
	exerciseWriteBarrierMutation()
	// The recursive helper adds one for every odd depth.
	const recursiveAdjustment = 600
	if got, want := exercise(true), 2*(13+23+17+13+17)+recursiveAdjustment; got != want {
		panic("address-taken pointer local lost across mutation, GC, or stack growth")
	}
	if got, want := exercise(false), 2*(11+23+19+17+11)+recursiveAdjustment; got != want {
		panic("address-taken pointer local lost across branch or repeated safepoints")
	}
}
