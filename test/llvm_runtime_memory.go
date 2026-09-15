// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "unsafe"

//go:linkname rawMove runtime.memmove
func rawMove(dst, src unsafe.Pointer, n uintptr)

//go:linkname rawClear runtime.memclrNoHeapPointers
func rawClear(dst unsafe.Pointer, n uintptr)

//go:linkname rawEqual runtime.memequal
func rawEqual(a, b unsafe.Pointer, n uintptr) bool

func main() {
	// Zero-sized operations accept nil pointers: the attribute model must not
	// imply nonnull or unconditional dereferenceability.
	rawMove(nil, nil, 0)
	rawClear(nil, 0)
	if !rawEqual(nil, nil, 0) {
		panic("empty comparison")
	}
	for _, n := range []int{1, 7, 8, 31, 32, 128, 1024} {
		// Exercise both overlap directions, identical pointers, and distinct
		// ranges. In particular, readonly on src must not imply noalias.
		for _, offsets := range [][2]int{{0, 3}, {3, 0}, {1, 1}, {0, 1100}} {
			var data, before, want [2200]byte
			for i := range data {
				data[i] = byte(i*17 + 3)
				before[i] = data[i]
				want[i] = data[i]
			}
			dst, src := offsets[0], offsets[1]
			for i := 0; i < n; i++ {
				want[dst+i] = before[src+i]
			}
			rawMove(unsafe.Pointer(&data[dst]), unsafe.Pointer(&data[src]), uintptr(n))
			for i := range data {
				if data[i] != want[i] {
					panic("overlapping copy or source preservation")
				}
			}
			if !rawEqual(unsafe.Pointer(&data[0]), unsafe.Pointer(&want[0]), uintptr(len(data))) {
				panic("equal buffers")
			}
			want[dst] ^= 1
			if rawEqual(unsafe.Pointer(&data[0]), unsafe.Pointer(&want[0]), uintptr(len(data))) {
				panic("unequal buffers")
			}
			want[dst] ^= 1
			rawClear(unsafe.Pointer(&data[dst]), uintptr(n))
			for i := range data {
				expected := want[i]
				if i >= dst && i < dst+n {
					expected = 0
				}
				if data[i] != expected {
					panic("clear destination or adjacent memory")
				}
			}
		}
	}
}
