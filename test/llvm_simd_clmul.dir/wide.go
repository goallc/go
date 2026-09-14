// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"simd/archsimd"
)

//go:noinline
func multiply256(x, y [8]uint64, selection int) (out [8]uint64) {
	if !archsimd.X86.VPCLMULQDQ() {
		return
	}
	a, b := archsimd.LoadUint64x4(x[:4]), archsimd.LoadUint64x4(y[:4])
	var product archsimd.Uint64x4
	switch selection {
	case 0:
		product = a.CarrylessMultiplyEven(b)
	case 1:
		product = a.CarrylessMultiplyOddEven(b)
	case 2:
		product = a.CarrylessMultiplyEvenOdd(b)
	case 3:
		product = a.CarrylessMultiplyOdd(b)
	}
	product.Store(out[:4])
	return
}

//go:noinline
func multiply512(x, y [8]uint64, selection int) (out [8]uint64) {
	if !archsimd.X86.AVX512VPCLMULQDQ() {
		return
	}
	a, b := archsimd.LoadUint64x8(x[:]), archsimd.LoadUint64x8(y[:])
	var product archsimd.Uint64x8
	switch selection {
	case 0:
		product = a.CarrylessMultiplyEven(b)
	case 1:
		product = a.CarrylessMultiplyOddEven(b)
	case 2:
		product = a.CarrylessMultiplyEvenOdd(b)
	case 3:
		product = a.CarrylessMultiplyOdd(b)
	}
	product.Store(out[:])
	return
}

func checkWide() {
	x := [8]uint64{0, 1, ^uint64(0), 1 << 63, 0xaaaaaaaaaaaaaaaa, 0x0123456789abcdef, 0x5555555555555555, 0xfedcba9876543210}
	y := [8]uint64{0xfedcba9876543210, 0x5555555555555555, 1 << 63, 0x0123456789abcdef, 1, ^uint64(0), 0xaaaaaaaaaaaaaaaa, 0}
	for _, test := range []struct {
		lanes   int
		enabled bool
		fn      func([8]uint64, [8]uint64, int) [8]uint64
	}{
		{4, archsimd.X86.VPCLMULQDQ(), multiply256},
		{8, archsimd.X86.AVX512VPCLMULQDQ(), multiply512},
	} {
		for selection := 0; selection < 4; selection++ {
			var want [8]uint64
			if test.enabled {
				for lane := 0; lane < test.lanes; lane += 2 {
					product := reference(x[lane+(selection&1)], y[lane+(selection>>1)])
					want[lane], want[lane+1] = product[0], product[1]
				}
			}
			if got := test.fn(x, y, selection); got != want {
				panic(fmt.Sprintf("width=%d selection=%d got=%x want=%x", test.lanes*64, selection, got, want))
			}
		}
		if os.Getenv("GOALLC_SIMD_CLMUL_TRACE") != "" {
			fmt.Printf("width=%d enabled=%v\n", test.lanes*64, test.enabled)
		}
	}
}
