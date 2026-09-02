//go:build amd64 || arm64

// run -goexperiment simd -llvm-package-only

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// Keep dependencies native: enabling the SIMD experiment exposes wider SIMD
// operations that the current LLVM lowering intentionally rejects in runtime
// and archsimd. The command-line package still uses LLVM, matching the scope
// of the former fixture while testing its vector ABI and stack growth.

import (
	"runtime"
	"simd/archsimd"
)

//go:noinline
func llvmVec128TouchFrame(p *[32 << 10]byte) byte {
	p[0] = 1
	p[len(p)-1] = 2
	return p[0] + p[len(p)-1]
}

// llvmVec128AddWithStackGrowth keeps both vector arguments live while using a
// frame larger than a fresh goroutine stack. It exercises the vector argument
// homes used by morestack and an ordinary vector return.
//
//go:noinline
func llvmVec128AddWithStackGrowth(x, y archsimd.Int8x16) archsimd.Int8x16 {
	var frame [32 << 10]byte
	if llvmVec128TouchFrame(&frame) != 3 {
		panic("bad stack frame")
	}
	return x.Add(y)
}

func main() {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}
	x := [16]int8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	y := [16]int8{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	var got [16]int8
	llvmVec128AddWithStackGrowth(
		archsimd.LoadInt8x16Array(&x),
		archsimd.LoadInt8x16Array(&y),
	).StoreArray(&got)
	for i, value := range got {
		if value != 16 {
			panic(i)
		}
	}
}
