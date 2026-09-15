// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "unsafe"

//go:noinline
func scaled(p unsafe.Pointer, index uintptr) unsafe.Pointer {
	return unsafe.Add(p, index<<4)
}

//go:noinline
func signed(p unsafe.Pointer, index int) unsafe.Pointer {
	return unsafe.Add(p, index<<4)
}

//go:noinline
func narrow(p unsafe.Pointer, index uint32) unsafe.Pointer {
	return unsafe.Add(p, index<<4)
}

//go:noinline
func dynamic(p unsafe.Pointer, index uintptr, shift uint) unsafe.Pointer {
	return unsafe.Add(p, index<<shift)
}

//go:noinline
func shared(p unsafe.Pointer, index uintptr) (unsafe.Pointer, uintptr) {
	offset := index << 4
	return unsafe.Add(p, offset), offset
}

func main() {
	var data [8][16]byte
	base := unsafe.Pointer(&data[0][0])
	middle := unsafe.Pointer(&data[4][0])
	bits := uint(unsafe.Sizeof(uintptr(0)) * 8)
	wrap := uintptr(1) << (bits - 4)
	for i := uintptr(0); i < 8; i++ {
		want := unsafe.Pointer(&data[i][0])
		if scaled(base, i) != want || scaled(base, wrap+i) != want {
			panic("scaled pointer offset lost word-sized wraparound")
		}
		if signed(middle, int(i)-4) != want {
			panic("scaled pointer offset lost a negative index")
		}
		if narrow(base, 1<<28+uint32(i)) != want {
			panic("scaled pointer offset lost narrow integer wraparound")
		}
		if dynamic(base, i, 4) != want || dynamic(base, i, bits) != base || dynamic(base, i, ^uint(0)) != base {
			panic("scaled pointer offset changed dynamic shift semantics")
		}
		p, offset := shared(base, wrap+i)
		if p != want || offset != i<<4 {
			panic("scaled pointer offset changed a shared offset")
		}
		*(*byte)(scaled(base, wrap+i)) = byte(i + 1)
		if data[i][0] != byte(i+1) {
			panic("scaled pointer does not address the expected element")
		}
	}
}
