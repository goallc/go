// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math"
	"simd/archsimd"
)

type element interface {
	int8 | int16 | int32 | int64 | float32 | float64
}

func same[T element](a, b T) bool {
	switch a := any(a).(type) {
	case float32:
		return math.Float32bits(a) == math.Float32bits(any(b).(float32))
	case float64:
		return math.Float64bits(a) == math.Float64bits(any(b).(float64))
	}
	return a == b
}

func checkNaN[T float32 | float64](x []T, bits uint64) {
	var want uint64
	for i, v := range x {
		if v != v {
			want |= uint64(1) << i
		}
	}
	if bits != want {
		panic("SIMD IsNaN mask mismatch")
	}
}

func check[T element](name string, lanes int, available bool, fn func([]T, []T, uint64, [3][]T)) {
	// Rotate the inputs so even two-lane vectors exercise every float pattern.
	for start := range 8 {
		checkInputs(name, lanes, start, available, fn)
	}
}

func checkInputs[T element](name string, lanes, start int, available bool, fn func([]T, []T, uint64, [3][]T)) {
	x, y := make([]T, lanes), make([]T, lanes)
	for i := range x {
		x[i] = T(37*i - 113 + 11*start)
		switch any(x[i]).(type) {
		case float32:
			patterns := [...]uint32{0, 1 << 31, 0x7fc12345, 0x7f800000, 0xff800000, 1, 0x3fc00000, 0xbf800000}
			x[i] = any(math.Float32frombits(patterns[(start+i)%len(patterns)])).(T)
		case float64:
			patterns := [...]uint64{0, 1 << 63, 0x7ff8123456789abc, 0x7ff0000000000000, 0xfff0000000000000, 1, 0x3ff8000000000000, 0xbff0000000000000}
			x[i] = any(math.Float64frombits(patterns[(start+i)%len(patterns)])).(T)
		}
	}
	for i := range y {
		y[i] = x[lanes-1-i]
	}
	masks := []uint64{0, ^uint64(0), 0xaaaaaaaaaaaaaaaa, 0x5555555555555555, 0x8000000000000001, 0x53f09a68c7e412bd}
	for i := range lanes {
		masks = append(masks, uint64(1)<<i, ^(uint64(1) << i))
	}
	for _, bits := range masks {
		out := [3][]T{make([]T, lanes), make([]T, lanes), make([]T, lanes)}
		want := [3][]T{make([]T, lanes), make([]T, lanes), make([]T, lanes)}
		if available {
			n := 0
			for i := range lanes {
				want[2][i] = y[i]
				if bits>>i&1 != 0 {
					want[0][n], want[1][i], want[2][i] = x[i], x[n], x[i]
					n++
				}
			}
		}
		fn(x, y, bits, out)
		for op := range out {
			for i, got := range out[op] {
				if !same(got, want[op][i]) {
					panic(fmt.Sprintf("%s op=%d mask=%#x lane=%d got=%v want=%v", name, op, bits, i, got, want[op][i]))
				}
			}
		}
	}
}

func main() {
	if archsimd.X86.AVX512() {
		for bits := range 256 {
			box := maskResult(uint8(bits))
			if box.tag != 0x12345678 || box.mask.ToBits() != uint8(bits)&15 {
				panic("mask aggregate return mismatch")
			}
		}
	}
	for bit := -1; bit < 256; bit++ {
		var x [32]byte
		if bit >= 0 {
			x[bit/8] = 1 << (bit % 8)
		}
		z128, z256 := zeros(x[:])
		if archsimd.X86.AVX() && (z128 != (bit < 0 || bit >= 128) || z256 != (bit < 0)) {
			panic("SIMD IsZero mismatch")
		}
	}
	check("Int8x16", 16, archsimd.X86.AVX512VBMI2(), int8x16)
	check("Int8x32", 32, archsimd.X86.AVX512VBMI2(), int8x32)
	check("Int8x64", 64, archsimd.X86.AVX512VBMI2(), int8x64)
	check("Int16x8", 8, archsimd.X86.AVX512VBMI2(), int16x8)
	check("Int16x16", 16, archsimd.X86.AVX512VBMI2(), int16x16)
	check("Int16x32", 32, archsimd.X86.AVX512VBMI2(), int16x32)
	check("Int32x4", 4, archsimd.X86.AVX512(), int32x4)
	check("Int32x8", 8, archsimd.X86.AVX512(), int32x8)
	check("Int32x16", 16, archsimd.X86.AVX512(), int32x16)
	check("Float32x4", 4, archsimd.X86.AVX512(), float32x4)
	check("Float32x8", 8, archsimd.X86.AVX512(), float32x8)
	check("Float32x16", 16, archsimd.X86.AVX512(), float32x16)
	check("Int64x2", 2, archsimd.X86.AVX512(), int64x2)
	check("Int64x4", 4, archsimd.X86.AVX512(), int64x4)
	check("Int64x8", 8, archsimd.X86.AVX512(), int64x8)
	check("Float64x2", 2, archsimd.X86.AVX512(), float64x2)
	check("Float64x4", 4, archsimd.X86.AVX512(), float64x4)
	check("Float64x8", 8, archsimd.X86.AVX512(), float64x8)
}
