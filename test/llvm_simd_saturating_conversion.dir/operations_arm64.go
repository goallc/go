// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build arm64

package main

import (
	"simd/archsimd"
)

//go:noinline
func SaturateToUint8Int16x8(x archsimd.Int16x8) archsimd.Uint8x16 {
	return x.SaturateToUint8()
}

//go:noinline
func SaturateToUint16Int32x4(x archsimd.Int32x4) archsimd.Uint16x8 {
	return x.SaturateToUint16()
}

//go:noinline
func SaturateToUint32Int64x2(x archsimd.Int64x2) archsimd.Uint32x4 {
	return x.SaturateToUint32()
}

func checkArch() {
	check("SaturateToUint8Int16x8", 16, 8, 8, 16, true, false, false, true, true, func(x, y []int16, got []uint8) {
		result := SaturateToUint8Int16x8(archsimd.LoadInt16x8(x))
		result.Store(got)
	})
	check("SaturateToUint16Int32x4", 32, 4, 16, 8, true, false, false, true, true, func(x, y []int32, got []uint16) {
		result := SaturateToUint16Int32x4(archsimd.LoadInt32x4(x))
		result.Store(got)
	})
	check("SaturateToUint32Int64x2", 64, 2, 32, 4, true, false, false, true, true, func(x, y []int64, got []uint32) {
		result := SaturateToUint32Int64x2(archsimd.LoadInt64x2(x))
		result.Store(got)
	})
}
