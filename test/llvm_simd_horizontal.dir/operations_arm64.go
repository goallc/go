// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func horizontalInt16x8(x, y, out []int16) {
	a, b := archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y)
	a.ConcatAddPairs(b).Store(out[0:8])
}

//go:noinline
func horizontalUint16x8(x, y, out []uint16) {
	a, b := archsimd.LoadUint16x8(x), archsimd.LoadUint16x8(y)
	a.ConcatAddPairs(b).Store(out[0:8])
}

//go:noinline
func horizontalInt32x4(x, y, out []int32) {
	a, b := archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y)
	a.ConcatAddPairs(b).Store(out[0:4])
}

//go:noinline
func horizontalUint32x4(x, y, out []uint32) {
	a, b := archsimd.LoadUint32x4(x), archsimd.LoadUint32x4(y)
	a.ConcatAddPairs(b).Store(out[0:4])
}

//go:noinline
func horizontalInt64x2(x, y, out []int64) {
	a, b := archsimd.LoadInt64x2(x), archsimd.LoadInt64x2(y)
	a.ConcatAddPairs(b).Store(out[0:2])
}

//go:noinline
func horizontalUint64x2(x, y, out []uint64) {
	a, b := archsimd.LoadUint64x2(x), archsimd.LoadUint64x2(y)
	a.ConcatAddPairs(b).Store(out[0:2])
}

//go:noinline
func horizontalFloat32x4(x, y, out []float32) {
	a, b := archsimd.LoadFloat32x4(x), archsimd.LoadFloat32x4(y)
	a.ConcatAddPairs(b).Store(out[0:4])
}

//go:noinline
func horizontalFloat64x2(x, y, out []float64) {
	a, b := archsimd.LoadFloat64x2(x), archsimd.LoadFloat64x2(y)
	a.ConcatAddPairs(b).Store(out[0:2])
}

func checkArch() {
	check("Int16x8", 8, 8, []int{0}, true, horizontalInt16x8)
	check("Uint16x8", 8, 8, []int{0}, true, horizontalUint16x8)
	check("Int32x4", 4, 4, []int{0}, true, horizontalInt32x4)
	check("Uint32x4", 4, 4, []int{0}, true, horizontalUint32x4)
	check("Int64x2", 2, 2, []int{0}, true, horizontalInt64x2)
	check("Uint64x2", 2, 2, []int{0}, true, horizontalUint64x2)
	check("Float32x4", 4, 4, []int{0}, true, horizontalFloat32x4)
	check("Float64x2", 2, 2, []int{0}, true, horizontalFloat64x2)
}
