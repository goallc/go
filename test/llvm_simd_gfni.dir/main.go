// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math/bits"
	"os"
	"simd/archsimd"
)

func mul(x, y byte) (z byte) {
	for y != 0 {
		if y&1 != 0 {
			z ^= x
		}
		x = x<<1 ^ (x>>7)*0x1b
		y >>= 1
	}
	return
}

func inverse(x byte) byte {
	y := byte(1)
	for n := 254; n != 0; n >>= 1 {
		if n&1 != 0 {
			y = mul(y, x)
		}
		x = mul(x, x)
	}
	return y
}

func affine(x byte, matrix uint64, b byte) byte {
	// Matrix row 0 is the most significant byte of the uint64 operand.
	for i := 0; i < 8; i++ {
		b ^= byte(bits.OnesCount8(x&byte(matrix>>uint(8*(7-i))))&1) << uint(i)
	}
	return b
}

func main() {
	enabled := archsimd.X86.AVX512GFNI()
	for _, test := range []struct {
		size int
		fn   func([64]byte, [64]byte, [8]uint64, uint8) [3][64]byte
	}{
		{16, gf128}, {32, gf256}, {64, gf512},
	} {
		// Exercise every byte, including the inverse of zero, at every width.
		for start := 0; start < 256; start += test.size {
			var x, y [64]byte
			matrices := [8]uint64{0x0102040810204080, 0, ^uint64(0), 0xf1e2d3c4b5a69788, 0x8040201008040201, 0x1133557799bbddff, 0x97ac39b1e8f2046d, 0x0804020180402010}
			var matrix [8]uint64
			for i := range matrix {
				matrix[i] = matrices[(i+start/test.size)%len(matrices)]
			}
			for i := range x {
				x[i] = byte(start + i)
				y[i] = byte(197*(start+i) + 13)
			}
			for _, b := range []byte{0, 0x63, 0xff} {
				var want [3][64]byte
				if enabled {
					for i := 0; i < test.size; i++ {
						want[0][i] = mul(x[i], y[i])
						want[1][i] = affine(x[i], matrix[i/8], b)
						want[2][i] = affine(inverse(x[i]), matrix[i/8], b)
					}
				}
				if got := test.fn(x, y, matrix, b); got != want {
					panic(fmt.Sprintf("width=%d start=%d b=%x got=%x want=%x", test.size*8, start, b, got, want))
				}
			}
		}
	}
	if os.Getenv("GOALLC_SIMD_GFNI_TRACE") != "" {
		fmt.Printf("AVX512GFNI=%v\n", enabled)
	}
}
