// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func horizontalInt16x8(x, y, out []int16) {
	if !archsimd.X86.AVX() {
		return
	}
	a, b := archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y)
	a.ConcatAddPairs(b).Store(out[0:8])
	a.ConcatSubPairs(b).Store(out[8:16])
	a.ConcatAddPairsSaturated(b).Store(out[16:24])
	a.ConcatSubPairsSaturated(b).Store(out[24:32])
}

//go:noinline
func horizontalInt16x16(x, y, out []int16) {
	if !archsimd.X86.AVX2() {
		return
	}
	a, b := archsimd.LoadInt16x16(x), archsimd.LoadInt16x16(y)
	a.ConcatAddPairsGrouped(b).Store(out[0:16])
	a.ConcatSubPairsGrouped(b).Store(out[16:32])
	a.ConcatAddPairsSaturatedGrouped(b).Store(out[32:48])
	a.ConcatSubPairsSaturatedGrouped(b).Store(out[48:64])
}

//go:noinline
func horizontalUint16x8(x, y, out []uint16) {
	if !archsimd.X86.AVX() {
		return
	}
	a, b := archsimd.LoadUint16x8(x), archsimd.LoadUint16x8(y)
	a.ConcatAddPairs(b).Store(out[0:8])
	a.ConcatSubPairs(b).Store(out[8:16])
}

//go:noinline
func horizontalUint16x16(x, y, out []uint16) {
	if !archsimd.X86.AVX2() {
		return
	}
	a, b := archsimd.LoadUint16x16(x), archsimd.LoadUint16x16(y)
	a.ConcatAddPairsGrouped(b).Store(out[0:16])
	a.ConcatSubPairsGrouped(b).Store(out[16:32])
}

//go:noinline
func horizontalInt32x4(x, y, out []int32) {
	if !archsimd.X86.AVX() {
		return
	}
	a, b := archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y)
	a.ConcatAddPairs(b).Store(out[0:4])
	a.ConcatSubPairs(b).Store(out[4:8])
}

//go:noinline
func horizontalInt32x8(x, y, out []int32) {
	if !archsimd.X86.AVX2() {
		return
	}
	a, b := archsimd.LoadInt32x8(x), archsimd.LoadInt32x8(y)
	a.ConcatAddPairsGrouped(b).Store(out[0:8])
	a.ConcatSubPairsGrouped(b).Store(out[8:16])
}

//go:noinline
func horizontalUint32x4(x, y, out []uint32) {
	if !archsimd.X86.AVX() {
		return
	}
	a, b := archsimd.LoadUint32x4(x), archsimd.LoadUint32x4(y)
	a.ConcatAddPairs(b).Store(out[0:4])
	a.ConcatSubPairs(b).Store(out[4:8])
}

//go:noinline
func horizontalUint32x8(x, y, out []uint32) {
	if !archsimd.X86.AVX2() {
		return
	}
	a, b := archsimd.LoadUint32x8(x), archsimd.LoadUint32x8(y)
	a.ConcatAddPairsGrouped(b).Store(out[0:8])
	a.ConcatSubPairsGrouped(b).Store(out[8:16])
}

//go:noinline
func horizontalFloat32x4(x, y, out []float32) {
	if !archsimd.X86.AVX() {
		return
	}
	a, b := archsimd.LoadFloat32x4(x), archsimd.LoadFloat32x4(y)
	a.ConcatAddPairs(b).Store(out[0:4])
	a.ConcatSubPairs(b).Store(out[4:8])
	a.AddOddSubEven(b).Store(out[8:12])
}

//go:noinline
func horizontalFloat32x8(x, y, out []float32) {
	if !archsimd.X86.AVX() {
		return
	}
	a, b := archsimd.LoadFloat32x8(x), archsimd.LoadFloat32x8(y)
	a.ConcatAddPairsGrouped(b).Store(out[0:8])
	a.ConcatSubPairsGrouped(b).Store(out[8:16])
	a.AddOddSubEven(b).Store(out[16:24])
}

//go:noinline
func horizontalFloat64x2(x, y, out []float64) {
	if !archsimd.X86.AVX() {
		return
	}
	a, b := archsimd.LoadFloat64x2(x), archsimd.LoadFloat64x2(y)
	a.ConcatAddPairs(b).Store(out[0:2])
	a.ConcatSubPairs(b).Store(out[2:4])
	a.AddOddSubEven(b).Store(out[4:6])
}

//go:noinline
func horizontalFloat64x4(x, y, out []float64) {
	if !archsimd.X86.AVX() {
		return
	}
	a, b := archsimd.LoadFloat64x4(x), archsimd.LoadFloat64x4(y)
	a.ConcatAddPairsGrouped(b).Store(out[0:4])
	a.ConcatSubPairsGrouped(b).Store(out[4:8])
	a.AddOddSubEven(b).Store(out[8:12])
}

func checkArch() {
	check("Int16x8", 8, 8, []int{0, 1, 2, 3}, archsimd.X86.AVX(), horizontalInt16x8)
	check("Int16x16", 16, 8, []int{0, 1, 2, 3}, archsimd.X86.AVX2(), horizontalInt16x16)
	check("Uint16x8", 8, 8, []int{0, 1}, archsimd.X86.AVX(), horizontalUint16x8)
	check("Uint16x16", 16, 8, []int{0, 1}, archsimd.X86.AVX2(), horizontalUint16x16)
	check("Int32x4", 4, 4, []int{0, 1}, archsimd.X86.AVX(), horizontalInt32x4)
	check("Int32x8", 8, 4, []int{0, 1}, archsimd.X86.AVX2(), horizontalInt32x8)
	check("Uint32x4", 4, 4, []int{0, 1}, archsimd.X86.AVX(), horizontalUint32x4)
	check("Uint32x8", 8, 4, []int{0, 1}, archsimd.X86.AVX2(), horizontalUint32x8)
	check("Float32x4", 4, 4, []int{0, 1, 4}, archsimd.X86.AVX(), horizontalFloat32x4)
	check("Float32x8", 8, 4, []int{0, 1, 4}, archsimd.X86.AVX(), horizontalFloat32x8)
	check("Float64x2", 2, 2, []int{0, 1, 4}, archsimd.X86.AVX(), horizontalFloat64x2)
	check("Float64x4", 4, 2, []int{0, 1, 4}, archsimd.X86.AVX(), horizontalFloat64x4)
}
