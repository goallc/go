//go:build amd64 || arm64

// run -goexperiment simd -llvm-package-only

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"runtime"
	"simd/archsimd"
)

//go:noinline
func composeAverageUnsigned(x, y archsimd.Uint8x16) archsimd.Uint8x16 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return x
	}
	return x.Average(y)
}

func main() {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}
	for xv := 0; xv < 256; xv++ {
		var x [16]uint8
		for i := range x {
			x[i] = uint8(xv)
		}
		for ybase := 0; ybase < 256; ybase += len(x) {
			var y [16]uint8
			for i := range y {
				y[i] = uint8(ybase + i)
			}
			result := composeAverageUnsigned(archsimd.LoadUint8x16Array(&x), archsimd.LoadUint8x16Array(&y))
			var got [16]uint8
			result.StoreArray(&got)
			for i := range got {
				want := uint8((xv + int(y[i]) + 1) / 2)
				if got[i] != want {
					panic(xv*256 + int(y[i]))
				}
			}
		}
	}
}
