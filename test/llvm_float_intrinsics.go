// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/subtle"
	"math"
	"math/bits"
	"sync/atomic"
)

//go:noinline
func sqrt64(x float64) float64 {
	return math.Sqrt(x)
}

//go:noinline
func abs64(x float64) float64 {
	return math.Abs(x)
}

//go:noinline
func trunc64(x float64) float64 {
	return math.Trunc(x)
}

//go:noinline
func floor64(x float64) float64 {
	return math.Floor(x)
}

//go:noinline
func ceil64(x float64) float64 {
	return math.Ceil(x)
}

//go:noinline
func roundAway64(x float64) float64 {
	return math.Round(x)
}

//go:noinline
func roundEven64(x float64) float64 {
	return math.RoundToEven(x)
}

//go:noinline
func fused64(x, y, z float64) float64 {
	return math.FMA(x, y, z)
}

//go:noinline
func min64(x, y float64) float64 {
	return min(x, y)
}

//go:noinline
func max64(x, y float64) float64 {
	return max(x, y)
}

//go:noinline
func rotate64(x uint64, count int) uint64 {
	return bits.RotateLeft64(x, count)
}

//go:noinline
func rotate32(x uint32, count int) uint32 {
	return bits.RotateLeft32(x, count)
}

//go:noinline
func bitLen64(x uint64) int {
	return bits.Len64(x)
}

//go:noinline
func bitLen32(x uint32) int {
	return bits.Len32(x)
}

//go:noinline
func trailingZeros64(x uint64) int {
	return bits.TrailingZeros64(x)
}

//go:noinline
func trailingZeros32(x uint32) int {
	return bits.TrailingZeros32(x)
}

//go:noinline
func trailingZeros16(x uint16) int {
	return bits.TrailingZeros16(x)
}

//go:noinline
func trailingZeros8(x uint8) int {
	return bits.TrailingZeros8(x)
}

//go:noinline
func populationCount64(x uint64) int {
	return bits.OnesCount64(x)
}

//go:noinline
func populationCount32(x uint32) int {
	return bits.OnesCount32(x)
}

//go:noinline
func populationCount16(x uint16) int {
	return bits.OnesCount16(x)
}

//go:noinline
func populationCount8(x uint8) int {
	return bits.OnesCount8(x)
}

//go:noinline
func add64Carry(x, y, carry uint64) (uint64, uint64) {
	return bits.Add64(x, y, carry)
}

//go:noinline
func sub64Borrow(x, y, borrow uint64) (uint64, uint64) {
	return bits.Sub64(x, y, borrow)
}

//go:noinline
func reverseBytes32(x uint32) uint32 {
	return bits.ReverseBytes32(x)
}

//go:noinline
func reverseBytes64(x uint64) uint64 {
	return bits.ReverseBytes64(x)
}

//go:noinline
func reverseBits64(x uint64) uint64 {
	return bits.Reverse64(x)
}

//go:noinline
func mul64HiLo(x, y uint64) (uint64, uint64) {
	return bits.Mul64(x, y)
}

//go:noinline
func selectInt(cond, x, y int) int {
	return subtle.ConstantTimeSelect(cond, x, y)
}

//go:noinline
func round64(x float64) float64 {
	return float64(x)
}

//go:noinline
func round32(x float32) float32 {
	return float32(x)
}

//go:noinline
func noContract(x, y, z float64) float64 {
	return float64(x*y) + z
}

//go:noinline
func noContract32(x, y, z float32) float32 {
	return float32(x*y) + z
}

func main() {
	if sqrt64(4) != 2 || !math.IsNaN(sqrt64(-1)) || !math.IsInf(sqrt64(math.Inf(1)), 1) {
		panic("sqrt semantics")
	}
	negZero := math.Float64frombits(1 << 63)
	if math.Float64bits(sqrt64(negZero)) != 1<<63 {
		panic("sqrt negative zero")
	}

	if math.Float64bits(abs64(negZero)) != 0 || !math.IsInf(abs64(math.Inf(-1)), 1) {
		panic("abs semantics")
	}
	nanBits := uint64(0xfff8000000001234)
	if got := math.Float64bits(abs64(math.Float64frombits(nanBits))); got != nanBits&^(1<<63) {
		panic("abs NaN payload")
	}

	for _, tc := range []struct {
		in, want uint64
	}{
		{0x0000000000000000, 0x0000000000000000}, // +0
		{0x8000000000000000, 0x8000000000000000}, // -0
		{0x400e000000000000, 0x4008000000000000}, // 3.75 -> 3
		{0xc00e000000000000, 0xc008000000000000}, // -3.75 -> -3
		{0x0000000000000001, 0x0000000000000000}, // smallest subnormal -> +0
		{0x8000000000000001, 0x8000000000000000}, // negative subnormal -> -0
		{0x43efffffffffffff, 0x43efffffffffffff}, // largest finite value below 2^64
		{0x7ff0000000000000, 0x7ff0000000000000}, // +Inf
		{0xfff0000000000000, 0xfff0000000000000}, // -Inf
	} {
		if got := math.Float64bits(trunc64(math.Float64frombits(tc.in))); got != tc.want {
			panic("trunc semantics")
		}
	}
	if !math.IsNaN(trunc64(math.Float64frombits(nanBits))) {
		panic("trunc NaN")
	}
	if floor64(3.75) != 3 || floor64(-3.25) != -4 || ceil64(3.25) != 4 || ceil64(-3.75) != -3 {
		panic("floor/ceil semantics")
	}
	if roundAway64(2.5) != 3 || roundAway64(-2.5) != -3 || roundEven64(2.5) != 2 || roundEven64(3.5) != 4 {
		panic("round semantics")
	}
	if fused64(1+1.0/(1<<27), 1-1.0/(1<<27), -1) != -1.0/(1<<54) {
		panic("fma semantics")
	}
	if math.Float64bits(min64(0, negZero)) != 1<<63 || math.Float64bits(max64(negZero, 0)) != 0 {
		panic("min/max signed zero")
	}
	if !math.IsNaN(min64(math.NaN(), 1)) || !math.IsNaN(max64(1, math.NaN())) {
		panic("min/max NaN")
	}

	if rotate64(0x0123456789abcdef, -8) != 0xef0123456789abcd || rotate32(0x12345678, 12) != 0x45678123 {
		panic("rotate semantics")
	}
	if bitLen64(0) != 0 || bitLen64(1<<63) != 64 || bitLen32(1<<31) != 32 {
		panic("bit length semantics")
	}
	if trailingZeros64(0) != 64 || trailingZeros64(1<<63) != 63 ||
		trailingZeros32(0) != 32 || trailingZeros32(1<<31) != 31 ||
		trailingZeros16(0) != 16 || trailingZeros16(1<<15) != 15 ||
		trailingZeros8(0) != 8 || trailingZeros8(1<<7) != 7 {
		panic("trailing zero semantics")
	}
	if populationCount64(^uint64(0)) != 64 || populationCount32(0xf0f0) != 8 ||
		populationCount16(0xaaaa) != 8 || populationCount8(0xf3) != 6 {
		panic("population count semantics")
	}
	if sum, carry := add64Carry(^uint64(0), 0, 1); sum != 0 || carry != 1 {
		panic("add carry semantics")
	}
	if sum, carry := add64Carry(41, 1, 0); sum != 42 || carry != 0 {
		panic("add without carry semantics")
	}
	if difference, borrow := sub64Borrow(0, 0, 1); difference != ^uint64(0) || borrow != 1 {
		panic("sub borrow semantics")
	}
	if difference, borrow := sub64Borrow(43, 1, 0); difference != 42 || borrow != 0 {
		panic("sub without borrow semantics")
	}
	if reverseBytes32(0x01234567) != 0x67452301 || reverseBytes64(0x0123456789abcdef) != 0xefcdab8967452301 {
		panic("byte swap semantics")
	}
	if reverseBits64(1) != 1<<63 || reverseBits64(0xf0) != 0x0f<<56 {
		panic("bit reverse semantics")
	}
	if high, low := mul64HiLo(^uint64(0), 2); high != 1 || low != ^uint64(1) {
		panic("wide multiply semantics")
	}
	if selectInt(1, 11, 22) != 11 || selectInt(0, 11, 22) != 22 {
		panic("conditional select semantics")
	}

	atomic64 := uint64(0xf3)
	if old := atomic.AndUint64(&atomic64, 0x0f); old != 0xf3 || atomic64 != 0x03 {
		panic("atomic and64 semantics")
	}
	if old := atomic.OrUint64(&atomic64, 0x80); old != 0x03 || atomic64 != 0x83 {
		panic("atomic or64 semantics")
	}
	atomic32 := uint32(0xf3)
	if old := atomic.AndUint32(&atomic32, 0x0f); old != 0xf3 || atomic32 != 0x03 {
		panic("atomic and32 semantics")
	}
	if old := atomic.OrUint32(&atomic32, 0x80); old != 0x03 || atomic32 != 0x83 {
		panic("atomic or32 semantics")
	}

	for _, bits := range []uint64{0, 1 << 63, 0x3ff0000000000000, nanBits} {
		if got := math.Float64bits(round64(math.Float64frombits(bits))); got != bits {
			panic("float64 rounding identity")
		}
	}
	for _, bits := range []uint32{0, 1 << 31, 0x3f800000, 0xffc01234} {
		if got := math.Float32bits(round32(math.Float32frombits(bits))); got != bits {
			panic("float32 rounding identity")
		}
	}

	// The exact product is 1-2^-54, exactly halfway between adjacent
	// binary64 values. It rounds to 1 before addition, producing +0. A fused
	// multiply-add would instead produce -2^-54.
	x := 1 + 1.0/(1<<27)
	y := 1 - 1.0/(1<<27)
	if got := noContract(x, y, -1); math.Float64bits(got) != 0 {
		panic("float64 conversion lost its rounding boundary")
	}
	// The exact product is 1-2^-26, which rounds to 1 in binary32 before
	// the addition. A fused multiply-add would instead produce -2^-26.
	x32 := float32(1 + 1.0/(1<<13))
	y32 := float32(1 - 1.0/(1<<13))
	if got := noContract32(x32, y32, -1); math.Float32bits(got) != 0 {
		panic("float32 conversion lost its rounding boundary")
	}
}
