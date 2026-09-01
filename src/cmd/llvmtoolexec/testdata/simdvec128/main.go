// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func touchFrame(p *[32 << 10]byte) byte {
	p[0] = 1
	p[len(p)-1] = 2
	return p[0] + p[len(p)-1]
}

// addWithStackGrowth keeps both vector arguments live while using a frame that
// is larger than a fresh goroutine stack. This exercises the Go ABI vector
// homes used by the morestack retry path as well as an ordinary vector return.
//
//go:noinline
func addWithStackGrowth(x, y archsimd.Int8x16) archsimd.Int8x16 {
	var frame [32 << 10]byte
	if touchFrame(&frame) != 3 {
		panic("bad stack frame")
	}
	return x.Add(y)
}

func main() {
	x := [16]int8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	y := [16]int8{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	var got [16]int8

	addWithStackGrowth(
		archsimd.LoadInt8x16Array(&x),
		archsimd.LoadInt8x16Array(&y),
	).StoreArray(&got)

	for i, value := range got {
		if value != 16 {
			panic(i)
		}
	}
}
