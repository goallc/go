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
	int8 | int16 | int32 | int64 | uint8 | uint16 | uint32 | uint64
}
type count interface{ int8 | int16 | int32 | int64 }

// reference uses scalar shifts and overflow bounds, not target intrinsics.
func reference(x uint64, bits uint, signed, saturated bool, count int8) uint64 {
	mask := ^uint64(0) >> (64 - bits)
	x &= mask
	s := int64(x<<(64-bits)) >> (64 - bits)
	n := int(count)
	if n < 0 {
		if signed {
			return uint64(s>>uint(-n)) & mask
		}
		return x >> uint(-n)
	}
	if saturated && x != 0 {
		if signed {
			max := int64(mask >> 1)
			min := -max - 1
			if n >= int(bits) || s > max>>uint(n) || s < min>>uint(n) {
				if s < 0 {
					return uint64(min) & mask
				}
				return uint64(max)
			}
		} else if n >= int(bits) || x > mask>>uint(n) {
			return mask
		}
	}
	return (x << uint(n)) & mask
}

func check[T integer, C count](bits uint, signed bool, shift func([]T, []C, []T, []T)) {
	lanes := 128 / int(bits)
	x, y, got, sat := make([]T, lanes), make([]C, lanes), make([]T, lanes), make([]T, lanes)
	mask := ^uint64(0) >> (64 - bits)
	values := []uint64{0, 1, 2, 3, mask, mask - 1, mask >> 1, (mask >> 1) - 1, 1 << (bits - 1), 1<<(bits-1) + 1, 0x5555555555555555, 0xaaaaaaaaaaaaaaaa}
	if bits == 8 {
		values = nil
		for v := uint64(0); v < 256; v++ {
			values = append(values, v)
		}
	}
	// Every signed low byte, with several different high-bit patterns.
	// Different lanes use different values and counts to catch lane mix-ups.
	for _, high := range []int64{0, 0x100, -0x100, 0x123456780000} {
		for c := -128; c < 128; c++ {
			for _, v := range values {
				for lane := range x {
					x[lane] = T(v + uint64(lane))
					y[lane] = C(high + int64(c+lane))
				}
				shift(x, y, got, sat)
				for lane := range x {
					for mode, result := range []T{got[lane], sat[lane]} {
						want := T(reference(uint64(x[lane]), bits, signed, mode == 1, int8(y[lane])))
						if result != want {
							panic(fmt.Sprintf("bits=%d signed=%v saturated=%v lane=%d x=%x count=%d got=%x want=%x", bits, signed, mode == 1, lane, x[lane], y[lane], result, want))
						}
					}
				}
			}
		}
	}
}

//go:noinline
func shiftInt8x16(x []int8, counts []int8, out, saturated []int8) {
	a, b := archsimd.LoadInt8x16(x), archsimd.LoadInt8x16(counts)
	a.Shift(b).Store(out)
	a.ShiftSaturated(b).Store(saturated)
}

//go:noinline
func shiftUint8x16(x []uint8, counts []int8, out, saturated []uint8) {
	a, b := archsimd.LoadUint8x16(x), archsimd.LoadInt8x16(counts)
	a.Shift(b).Store(out)
	a.ShiftSaturated(b).Store(saturated)
}

//go:noinline
func shiftInt16x8(x []int16, counts []int16, out, saturated []int16) {
	a, b := archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(counts)
	a.Shift(b).Store(out)
	a.ShiftSaturated(b).Store(saturated)
}

//go:noinline
func shiftUint16x8(x []uint16, counts []int16, out, saturated []uint16) {
	a, b := archsimd.LoadUint16x8(x), archsimd.LoadInt16x8(counts)
	a.Shift(b).Store(out)
	a.ShiftSaturated(b).Store(saturated)
}

//go:noinline
func shiftInt32x4(x []int32, counts []int32, out, saturated []int32) {
	a, b := archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(counts)
	a.Shift(b).Store(out)
	a.ShiftSaturated(b).Store(saturated)
}

//go:noinline
func shiftUint32x4(x []uint32, counts []int32, out, saturated []uint32) {
	a, b := archsimd.LoadUint32x4(x), archsimd.LoadInt32x4(counts)
	a.Shift(b).Store(out)
	a.ShiftSaturated(b).Store(saturated)
}

//go:noinline
func shiftInt64x2(x []int64, counts []int64, out, saturated []int64) {
	a, b := archsimd.LoadInt64x2(x), archsimd.LoadInt64x2(counts)
	a.Shift(b).Store(out)
	a.ShiftSaturated(b).Store(saturated)
}

//go:noinline
func shiftUint64x2(x []uint64, counts []int64, out, saturated []uint64) {
	a, b := archsimd.LoadUint64x2(x), archsimd.LoadInt64x2(counts)
	a.Shift(b).Store(out)
	a.ShiftSaturated(b).Store(saturated)
}

func main() {
	check(8, true, shiftInt8x16)
	check(8, false, shiftUint8x16)
	check(16, true, shiftInt16x8)
	check(16, false, shiftUint16x8)
	check(32, true, shiftInt32x4)
	check(32, false, shiftUint32x4)
	check(64, true, shiftInt64x2)
	check(64, false, shiftUint64x2)
}
