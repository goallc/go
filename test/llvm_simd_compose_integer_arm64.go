//go:build arm64

// run -goexperiment simd -llvm-package-only

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func composeAverageSigned(x, y archsimd.Int8x16) archsimd.Int8x16 {
	return x.Average(y)
}

//go:noinline
func composeLeadingSignBitsSigned(x archsimd.Int8x16) archsimd.Int8x16 {
	return x.LeadingSignBits()
}

//go:noinline
func composeLeadingSignBitsUnsigned(x archsimd.Uint8x16) archsimd.Uint8x16 {
	return x.LeadingSignBits()
}

func leadingSignBits8(x uint8) uint8 {
	sign := x >> 7
	var count uint8
	for bit := 6; bit >= 0; bit-- {
		if (x>>bit)&1 != sign {
			break
		}
		count++
	}
	return count
}

func main() {
	for xv := -128; xv < 128; xv++ {
		var x [16]int8
		for i := range x {
			x[i] = int8(xv)
		}
		for ybase := -128; ybase < 128; ybase += len(x) {
			var y [16]int8
			for i := range y {
				y[i] = int8(ybase + i)
			}
			average := composeAverageSigned(archsimd.LoadInt8x16Array(&x), archsimd.LoadInt8x16Array(&y))
			var got [16]int8
			average.StoreArray(&got)
			for i := range got {
				want := int8((xv + int(y[i]) + 1) >> 1)
				if got[i] != want {
					panic((xv+128)*256 + int(y[i]) + 128)
				}
			}
		}
	}

	x := [16]int8{-128, -127, -65, -64, -3, -2, -1, 0, 1, 2, 3, 63, 64, 65, 126, 127}
	signed := composeLeadingSignBitsSigned(archsimd.LoadInt8x16Array(&x))
	var gotSigned [16]int8
	signed.StoreArray(&gotSigned)
	for i := range gotSigned {
		if gotSigned[i] != int8(leadingSignBits8(uint8(x[i]))) {
			panic(16 + i)
		}
	}

	ux := [16]uint8{0, 1, 2, 3, 63, 64, 65, 126, 127, 128, 129, 192, 253, 254, 255, 42}
	unsigned := composeLeadingSignBitsUnsigned(archsimd.LoadUint8x16Array(&ux))
	var gotUnsigned [16]uint8
	unsigned.StoreArray(&gotUnsigned)
	for i := range gotUnsigned {
		if gotUnsigned[i] != leadingSignBits8(ux[i]) {
			panic(32 + i)
		}
	}
}
