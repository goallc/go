// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"runtime"
	"simd/archsimd"
)

//go:noinline
func SaturateToInt8Int16x8(x archsimd.Int16x8) archsimd.Int8x16 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Int8x16{}
	}
	return x.SaturateToInt8()
}

//go:noinline
func SaturateToInt16Int32x4(x archsimd.Int32x4) archsimd.Int16x8 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Int16x8{}
	}
	return x.SaturateToInt16()
}

//go:noinline
func SaturateToInt32Int64x2(x archsimd.Int64x2) archsimd.Int32x4 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Int32x4{}
	}
	return x.SaturateToInt32()
}

//go:noinline
func SaturateToUint8Uint16x8(x archsimd.Uint16x8) archsimd.Uint8x16 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Uint8x16{}
	}
	return x.SaturateToUint8()
}

//go:noinline
func SaturateToUint16Uint32x4(x archsimd.Uint32x4) archsimd.Uint16x8 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Uint16x8{}
	}
	return x.SaturateToUint16()
}

//go:noinline
func SaturateToUint32Uint64x2(x archsimd.Uint64x2) archsimd.Uint32x4 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Uint32x4{}
	}
	return x.SaturateToUint32()
}

func checkCommon() {
	check("SaturateToInt8Int16x8", 16, 8, 8, 16, true, true, false, runtime.GOARCH != "amd64" || archsimd.X86.AVX(), runtime.GOARCH != "amd64" || archsimd.X86.AVX512(), func(x, y []int16, got []int8) {
		result := SaturateToInt8Int16x8(archsimd.LoadInt16x8(x))
		result.Store(got)
	})
	check("SaturateToInt16Int32x4", 32, 4, 16, 8, true, true, false, runtime.GOARCH != "amd64" || archsimd.X86.AVX(), runtime.GOARCH != "amd64" || archsimd.X86.AVX512(), func(x, y []int32, got []int16) {
		result := SaturateToInt16Int32x4(archsimd.LoadInt32x4(x))
		result.Store(got)
	})
	check("SaturateToInt32Int64x2", 64, 2, 32, 4, true, true, false, runtime.GOARCH != "amd64" || archsimd.X86.AVX(), runtime.GOARCH != "amd64" || archsimd.X86.AVX512(), func(x, y []int64, got []int32) {
		result := SaturateToInt32Int64x2(archsimd.LoadInt64x2(x))
		result.Store(got)
	})
	check("SaturateToUint8Uint16x8", 16, 8, 8, 16, false, false, false, runtime.GOARCH != "amd64" || archsimd.X86.AVX(), runtime.GOARCH != "amd64" || archsimd.X86.AVX512(), func(x, y []uint16, got []uint8) {
		result := SaturateToUint8Uint16x8(archsimd.LoadUint16x8(x))
		result.Store(got)
	})
	check("SaturateToUint16Uint32x4", 32, 4, 16, 8, false, false, false, runtime.GOARCH != "amd64" || archsimd.X86.AVX(), runtime.GOARCH != "amd64" || archsimd.X86.AVX512(), func(x, y []uint32, got []uint16) {
		result := SaturateToUint16Uint32x4(archsimd.LoadUint32x4(x))
		result.Store(got)
	})
	check("SaturateToUint32Uint64x2", 64, 2, 32, 4, false, false, false, runtime.GOARCH != "amd64" || archsimd.X86.AVX(), runtime.GOARCH != "amd64" || archsimd.X86.AVX512(), func(x, y []uint64, got []uint32) {
		result := SaturateToUint32Uint64x2(archsimd.LoadUint64x2(x))
		result.Store(got)
	})
}
