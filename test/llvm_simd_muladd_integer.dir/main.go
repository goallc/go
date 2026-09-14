// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build arm64

package main

import (
	"fmt"
	"simd/archsimd"
)

type integer interface {
	int8 | int16 | int32 | uint8 | uint16 | uint32
}

func check[T integer](bits uint, mulAdd func([]T, []T, []T, []T)) {
	lanes := 128 / int(bits)
	x, y, z, out := make([]T, lanes), make([]T, lanes), make([]T, lanes), make([]T, lanes)
	mask := uint64(1)<<bits - 1
	values := []uint64{0, 1, 2, 3, mask, mask - 1, mask >> 1, (mask >> 1) - 1, 1 << (bits - 1), 1<<(bits-1) + 1, 0x55555555 & mask, 0xaaaaaaaa & mask}
	if bits == 8 {
		values = nil
		for v := uint64(0); v < 256; v++ {
			values = append(values, v)
		}
	} else {
		for bit := uint(0); bit < bits; bit++ {
			values = append(values, uint64(1)<<bit, (uint64(1)<<bit)-1)
		}
	}
	for _, a := range values {
		for _, b := range values {
			for _, c := range []uint64{0, 1, 3, mask, mask >> 1, 1 << (bits - 1), a, b} {
				for lane := range x {
					x[lane], y[lane], z[lane] = T(a+uint64(lane)), T(b-uint64(lane)), T(c^uint64(lane))
				}
				mulAdd(x, y, z, out)
				for lane := range x {
					// Widen the unsigned lane bit patterns before computing the result.
					// Even the 32-bit product plus accumulator fits in uint64.
					want := T((uint64(x[lane])&mask)*(uint64(y[lane])&mask) + (uint64(z[lane]) & mask))
					if out[lane] != want {
						panic(fmt.Sprintf("type=%T lane=%d x=%x y=%x z=%x got=%x want=%x", x[lane], lane, x[lane], y[lane], z[lane], out[lane], want))
					}
				}
			}
		}
	}
}

//go:noinline
func mulAddInt8x16(x, y, z, out []int8) {
	a, b, c := archsimd.LoadInt8x16(x), archsimd.LoadInt8x16(y), archsimd.LoadInt8x16(z)
	a.MulAdd(b, c).Store(out)
}

//go:noinline
func mulAddUint8x16(x, y, z, out []uint8) {
	a, b, c := archsimd.LoadUint8x16(x), archsimd.LoadUint8x16(y), archsimd.LoadUint8x16(z)
	a.MulAdd(b, c).Store(out)
}

//go:noinline
func mulAddInt16x8(x, y, z, out []int16) {
	a, b, c := archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y), archsimd.LoadInt16x8(z)
	a.MulAdd(b, c).Store(out)
}

//go:noinline
func mulAddUint16x8(x, y, z, out []uint16) {
	a, b, c := archsimd.LoadUint16x8(x), archsimd.LoadUint16x8(y), archsimd.LoadUint16x8(z)
	a.MulAdd(b, c).Store(out)
}

//go:noinline
func mulAddInt32x4(x, y, z, out []int32) {
	a, b, c := archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y), archsimd.LoadInt32x4(z)
	a.MulAdd(b, c).Store(out)
}

//go:noinline
func mulAddUint32x4(x, y, z, out []uint32) {
	a, b, c := archsimd.LoadUint32x4(x), archsimd.LoadUint32x4(y), archsimd.LoadUint32x4(z)
	a.MulAdd(b, c).Store(out)
}

func main() {
	check(8, mulAddInt8x16)
	check(8, mulAddUint8x16)
	check(16, mulAddInt16x8)
	check(16, mulAddUint16x8)
	check(32, mulAddInt32x4)
	check(32, mulAddUint32x4)
}
