//go:build amd64 || arm64

// run -goexperiment simd -godebug simd=+128 -llvm-package-only

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd"

//go:noinline
func llvmPortableSIMDAdd(x, y simd.Int8s) simd.Int8s {
	return x.Add(y)
}

func main() {
	if simd.VectorBitSize() != 128 || simd.Emulated() {
		panic("portable SIMD did not select the hardware 128-bit variant")
	}
	lanes := simd.Int8s{}.Len()
	x := make([]int8, lanes)
	y := make([]int8, lanes)
	got := make([]int8, lanes)
	for i := range lanes {
		x[i] = int8(i)
		y[i] = int8(lanes - i)
	}
	llvmPortableSIMDAdd(simd.LoadInt8s(x), simd.LoadInt8s(y)).Store(got)
	for i, value := range got {
		if value != int8(lanes) {
			panic(i)
		}
	}
}
