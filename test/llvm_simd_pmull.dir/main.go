// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build arm64

package main

import (
	"fmt"
	"os"
	"simd/archsimd"
)

//go:noinline
func multiply(x, y [2]uint64, selection int) (out [2]uint64) {
	if !archsimd.ARM64.PMULL() {
		return
	}
	a, b := archsimd.LoadUint64x2(x[:]), archsimd.LoadUint64x2(y[:])
	var product archsimd.Uint64x2
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

// Polynomial multiplication over GF(2), independently of target intrinsics.
func reference(x, y uint64) (out [2]uint64) {
	for bit := uint(0); bit < 64; bit++ {
		if y>>bit&1 != 0 {
			out[0] ^= x << bit
			if bit != 0 {
				out[1] ^= x >> (64 - bit)
			}
		}
	}
	return
}

func main() {
	data := []uint64{0, 1, ^uint64(0), 0xaaaaaaaaaaaaaaaa, 0x5555555555555555, 0x0123456789abcdef}
	for bit := uint(0); bit < 64; bit++ {
		data = append(data, uint64(1)<<bit)
	}
	enabled := archsimd.ARM64.PMULL()
	for _, a := range data {
		for _, b := range data {
			x, y := [2]uint64{a, ^a}, [2]uint64{b, ^b}
			for selection := 0; selection < 4; selection++ {
				var want [2]uint64
				if enabled {
					want = reference(x[selection&1], y[selection>>1])
				}
				if got := multiply(x, y, selection); got != want {
					panic(fmt.Sprintf("selection=%d x=%x y=%x got=%x want=%x", selection, x, y, got, want))
				}
			}
		}
	}
	if os.Getenv("GOALLC_SIMD_PMULL_TRACE") == "1" {
		fmt.Printf("pmull: enabled=%v products=%d\n", enabled, len(data)*len(data)*4)
	}
}
