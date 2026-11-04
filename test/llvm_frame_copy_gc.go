// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

type cell struct {
	value int
	space [63]uintptr
}

type tail struct {
	pointer *cell
	words   [8]uintptr
}

// The seven padding bytes between flag and data must not cause SROA to
// combine a pointer byte with the padding in an integer or byte-array slice.
type record struct {
	flag bool
	data tail
}

var sink uintptr

//go:noinline
func sumFrame(frame *[256]uintptr) uintptr {
	var sum uintptr
	for _, value := range frame {
		sum += value
	}
	return sum
}

//go:noinline
func grow(depth int) {
	var frame [256]uintptr
	frame[depth&255] = uintptr(depth + 1)
	if depth == 0 {
		runtime.GC()
	} else {
		grow(depth - 1)
	}
	sink += sumFrame(&frame)
}

// Copy subobjects, cross a safepoint with the snapshot, then copy the whole
// object to a fixed ABI result. The source is dead before the stack grows.
//
//go:noinline
func clone(source *record) record {
	var result record
	result.flag = source.flag
	result.data = source.data
	grow(128)
	return result
}

//go:noinline
func newCell(value int) *cell {
	return &cell{value: value}
}

//go:noinline
func heapSnapshot(value int) record {
	source := record{flag: true, data: tail{pointer: newCell(value)}}
	for i := range source.data.words {
		source.data.words[i] = uintptr(value + i)
	}
	return clone(&source)
}

//go:noinline
func stackSnapshot(value int) {
	local := cell{value: value}
	source := record{flag: true, data: tail{pointer: &local}}
	got := clone(&source)
	if !got.flag || got.data.pointer != &local || got.data.pointer.value != value {
		panic("padded copy lost a relocated stack pointer")
	}
}

//go:noinline
func check(value record, want int) {
	if !value.flag || value.data.pointer.value != want {
		panic("padded copy lost a heap pointer")
	}
	for i, word := range value.data.words {
		if word != uintptr(want+i) {
			panic("padded copy changed non-pointer bytes")
		}
	}
}

func main() {
	for iteration := 0; iteration < 4; iteration++ {
		done := make(chan bool)
		go func() {
			stackSnapshot(71 + iteration)
			var values [3]record
			values[0] = heapSnapshot(101 + iteration)
			values[1] = heapSnapshot(211 + iteration)
			// Preserve memmove overlap semantics for arrays of padded objects.
			copy(values[1:], values[:2])
			runtime.GC()
			check(values[1], 101+iteration)
			check(values[2], 211+iteration)
			done <- true
		}()
		<-done
	}
}
