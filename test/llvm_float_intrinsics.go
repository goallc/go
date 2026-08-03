// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "math"

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
