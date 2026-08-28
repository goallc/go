// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math"
	"runtime"
)

//go:noinline
func contract64Add(a, b, c float64) float64 { return a*b + c }

//go:noinline
func contract64MulSub(a, b, c float64) float64 { return a*b - c }

//go:noinline
func contract64SubMul(a, b, c float64) float64 { return c - a*b }

//go:noinline
func contract64NegMulAdd(a, b, c float64) float64 { return -(a * b) + c }

//go:noinline
func contract64NegMulSub(a, b, c float64) float64 { return -(a * b) - c }

//go:noinline
func contract32Add(a, b, c float32) float32 { return a*b + c }

//go:noinline
func contract32MulSub(a, b, c float32) float32 { return a*b - c }

//go:noinline
func contract32SubMul(a, b, c float32) float32 { return c - a*b }

//go:noinline
func contract32NegMulAdd(a, b, c float32) float32 { return -(a * b) + c }

//go:noinline
func contract32NegMulSub(a, b, c float32) float32 { return -(a * b) - c }

//go:noinline
func round64Boundary(a, b, c float64) float64 { return float64(a*b) + c }

//go:noinline
func round32Boundary(a, b, c float32) float32 { return float32(a*b) + c }

var inputs64 = [][3]uint64{
	{0x3ff0000002000000, 0x3feffffffc000000, 0xbff0000000000000}, // fused and separate differ
	{0x7fefffffffffffff, 0x4000000000000000, 0xffefffffffffffff}, // overflow cancellation
	{0x0000000000000001, 0x3ff0000000000000, 0x8000000000000000}, // subnormal and -0
	{0x8000000000000000, 0x4000000000000000, 0x8000000000000000}, // -0 product and -0
	{0x7ff0000000000000, 0x4000000000000000, 0x3ff0000000000000}, // +Inf
	{0x7ff8123412345678, 0x3ff0000000000000, 0x4000000000000000}, // quiet NaN payload
}

var inputs32 = [][3]uint32{
	{0x3f800400, 0x3f7ff800, 0xbf800000}, // fused and separate differ
	{0x7f7fffff, 0x40000000, 0xff7fffff}, // overflow cancellation
	{0x00000001, 0x3f800000, 0x80000000}, // subnormal and -0
	{0x80000000, 0x40000000, 0x80000000}, // -0 product and -0
	{0x7f800000, 0x40000000, 0x3f800000}, // +Inf
	{0x7fc12345, 0x3f800000, 0x40000000}, // quiet NaN payload
}

func fma32(a, b, c float32) float32 {
	// A float32 product has at most 48 significant bits, so these test cases
	// are exactly representable through the float64 FMA before rounding once
	// to float32.
	return float32(math.FMA(float64(a), float64(b), float64(c)))
}

func same64Bits(got, want uint64) bool {
	if math.IsNaN(math.Float64frombits(got)) && math.IsNaN(math.Float64frombits(want)) {
		// LLVM and the native ARM64 backend may commute the two multiply
		// operands. AArch64 preserves the NaN payload but the sign selected by
		// an FMSUB variant is not stable under that legal operand choice.
		return got&^(uint64(1)<<63) == want&^(uint64(1)<<63)
	}
	return got == want
}

func same32Bits(got, want uint32) bool {
	if math.IsNaN(float64(math.Float32frombits(got))) && math.IsNaN(float64(math.Float32frombits(want))) {
		return got&^(uint32(1)<<31) == want&^(uint32(1)<<31)
	}
	return got == want
}

func main() {
	if runtime.GOARCH != "arm64" {
		return
	}

	forms64 := []struct {
		name string
		got  func(float64, float64, float64) float64
		want func(float64, float64, float64) float64
	}{
		{"add", contract64Add, func(a, b, c float64) float64 { return math.FMA(a, b, c) }},
		{"mul-sub", contract64MulSub, func(a, b, c float64) float64 { return math.FMA(a, b, -c) }},
		{"sub-mul", contract64SubMul, func(a, b, c float64) float64 { return math.FMA(-a, b, c) }},
		{"neg-mul-add", contract64NegMulAdd, func(a, b, c float64) float64 { return math.FMA(-a, b, c) }},
		{"neg-mul-sub", contract64NegMulSub, func(a, b, c float64) float64 { return math.FMA(-a, b, -c) }},
	}
	for _, form := range forms64 {
		for _, bits := range inputs64 {
			a := math.Float64frombits(bits[0])
			b := math.Float64frombits(bits[1])
			c := math.Float64frombits(bits[2])
			got := math.Float64bits(form.got(a, b, c))
			want := math.Float64bits(form.want(a, b, c))
			if !same64Bits(got, want) {
				panic(fmt.Sprintf("float64 %s: got %#016x, want %#016x for %#x %#x %#x", form.name, got, want, bits[0], bits[1], bits[2]))
			}
		}
	}

	forms32 := []struct {
		name string
		got  func(float32, float32, float32) float32
		want func(float32, float32, float32) float32
	}{
		{"add", contract32Add, func(a, b, c float32) float32 { return fma32(a, b, c) }},
		{"mul-sub", contract32MulSub, func(a, b, c float32) float32 { return fma32(a, b, -c) }},
		{"sub-mul", contract32SubMul, func(a, b, c float32) float32 { return fma32(-a, b, c) }},
		{"neg-mul-add", contract32NegMulAdd, func(a, b, c float32) float32 { return fma32(-a, b, c) }},
		{"neg-mul-sub", contract32NegMulSub, func(a, b, c float32) float32 { return fma32(-a, b, -c) }},
	}
	for _, form := range forms32 {
		for _, bits := range inputs32 {
			a := math.Float32frombits(bits[0])
			b := math.Float32frombits(bits[1])
			c := math.Float32frombits(bits[2])
			got := math.Float32bits(form.got(a, b, c))
			want := math.Float32bits(form.want(a, b, c))
			if !same32Bits(got, want) {
				panic(fmt.Sprintf("float32 %s: got %#08x, want %#08x for %#x %#x %#x", form.name, got, want, bits[0], bits[1], bits[2]))
			}
		}
	}

	// Explicit conversions preserve the intervening rounding. These inputs
	// would produce a non-zero low bit if they were accidentally contracted.
	x64 := math.Float64frombits(inputs64[0][0])
	y64 := math.Float64frombits(inputs64[0][1])
	if got := math.Float64bits(round64Boundary(x64, y64, -1)); got != 0 {
		panic(fmt.Sprintf("float64 rounding boundary: got %#016x, want +0", got))
	}
	x32 := math.Float32frombits(inputs32[0][0])
	y32 := math.Float32frombits(inputs32[0][1])
	if got := math.Float32bits(round32Boundary(x32, y32, -1)); got != 0 {
		panic(fmt.Sprintf("float32 rounding boundary: got %#08x, want +0", got))
	}
}
