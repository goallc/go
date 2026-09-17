//go:build arm64

// run -goexperiment simd

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"math"
	"simd/archsimd"
)

//go:noinline
func reduceInt8(x archsimd.Int8x16) (int8, int8, int8) {
	return x.ReduceSum(), x.ReduceMax(), x.ReduceMin()
}

//go:noinline
func reduceInt16(x archsimd.Int16x8) (int16, int16, int16) {
	return x.ReduceSum(), x.ReduceMax(), x.ReduceMin()
}

//go:noinline
func reduceInt32(x archsimd.Int32x4) (int32, int32, int32) {
	return x.ReduceSum(), x.ReduceMax(), x.ReduceMin()
}

//go:noinline
func reduceUint8(x archsimd.Uint8x16) (uint8, uint8, uint8) {
	return x.ReduceSum(), x.ReduceMax(), x.ReduceMin()
}

//go:noinline
func reduceUint16(x archsimd.Uint16x8) (uint16, uint16, uint16) {
	return x.ReduceSum(), x.ReduceMax(), x.ReduceMin()
}

//go:noinline
func reduceUint32(x archsimd.Uint32x4) (uint32, uint32, uint32) {
	return x.ReduceSum(), x.ReduceMax(), x.ReduceMin()
}

//go:noinline
func reduceFloat32(x archsimd.Float32x4) (float32, float32) {
	return x.ReduceMax(), x.ReduceMin()
}

func main() {
	ints8 := [16]int8{-8, -7, -6, -5, -4, -3, -2, -1, 0, 1, 2, 3, 4, 5, 6, 7}
	if sum, max, min := reduceInt8(archsimd.LoadInt8x16Array(&ints8)); sum != -8 || max != 7 || min != -8 {
		panic("int8 reduction")
	}
	ints16 := [8]int16{-300, 20, 500, -40, 60, -70, 80, -90}
	if sum, max, min := reduceInt16(archsimd.LoadInt16x8Array(&ints16)); sum != 160 || max != 500 || min != -300 {
		panic("int16 reduction")
	}
	ints32 := [4]int32{-1_000_000, 200, 3_000_000, -40}
	if sum, max, min := reduceInt32(archsimd.LoadInt32x4Array(&ints32)); sum != 2_000_160 || max != 3_000_000 || min != -1_000_000 {
		panic("int32 reduction")
	}

	uints8 := [16]uint8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	if sum, max, min := reduceUint8(archsimd.LoadUint8x16Array(&uints8)); sum != 136 || max != 16 || min != 1 {
		panic("uint8 reduction")
	}
	uints16 := [8]uint16{300, 20, 500, 40, 60, 70, 80, 90}
	if sum, max, min := reduceUint16(archsimd.LoadUint16x8Array(&uints16)); sum != 1160 || max != 500 || min != 20 {
		panic("uint16 reduction")
	}
	uints32 := [4]uint32{1_000_000, 200, 3_000_000, 40}
	if sum, max, min := reduceUint32(archsimd.LoadUint32x4Array(&uints32)); sum != 4_000_240 || max != 3_000_000 || min != 40 {
		panic("uint32 reduction")
	}

	floats := [4]float32{-2.5, 4.5, -8, 1}
	if max, min := reduceFloat32(archsimd.LoadFloat32x4Array(&floats)); max != 4.5 || min != -8 {
		panic("float32 reduction")
	}
	nan := math.Float32frombits(0x7fc00001)
	floats = [4]float32{1, nan, -2, 3}
	if max, min := reduceFloat32(archsimd.LoadFloat32x4Array(&floats)); !math.IsNaN(float64(max)) || !math.IsNaN(float64(min)) {
		panic("float32 NaN reduction")
	}
	negativeZero := math.Float32frombits(1 << 31)
	floats = [4]float32{negativeZero, 0, negativeZero, 0}
	if max, min := reduceFloat32(archsimd.LoadFloat32x4Array(&floats)); math.Signbit(float64(max)) || !math.Signbit(float64(min)) {
		panic("float32 signed-zero reduction")
	}
}
