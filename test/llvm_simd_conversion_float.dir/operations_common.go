// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"runtime"
	"simd/archsimd"
)

//go:noinline
func ConvertToFloat32Float64x2(x archsimd.Float64x2) archsimd.Float32x4 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return archsimd.Float32x4{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func ConvertToFloat32Int32x4(x archsimd.Int32x4) archsimd.Float32x4 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return archsimd.Float32x4{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func ConvertToFloat32Uint32x4(x archsimd.Uint32x4) archsimd.Float32x4 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Float32x4{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func ConvertToFloat64Int64x2(x archsimd.Int64x2) archsimd.Float64x2 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Float64x2{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func ConvertToFloat64Uint64x2(x archsimd.Uint64x2) archsimd.Float64x2 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Float64x2{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func ConvertToInt32Float32x4(x archsimd.Float32x4) archsimd.Int32x4 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return archsimd.Int32x4{}
	}
	return x.ConvertToInt32()
}

//go:noinline
func ConvertToInt64Float64x2(x archsimd.Float64x2) archsimd.Int64x2 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Int64x2{}
	}
	return x.ConvertToInt64()
}

//go:noinline
func ConvertToUint32Float32x4(x archsimd.Float32x4) archsimd.Uint32x4 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Uint32x4{}
	}
	return x.ConvertToUint32()
}

//go:noinline
func ConvertToUint64Float64x2(x archsimd.Float64x2) archsimd.Uint64x2 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Uint64x2{}
	}
	return x.ConvertToUint64()
}

func checkCommon() {
	check("ConvertToFloat32Float64x2", "float", 64, 2, "float", 32, 4, runtime.GOARCH != "amd64" || archsimd.X86.AVX(), runtime.GOARCH != "amd64" || archsimd.X86.AVX(), func(x []float64, got []float32) {
		result := ConvertToFloat32Float64x2(archsimd.LoadFloat64x2(x))
		result.Store(got)
	})
	check("ConvertToFloat32Int32x4", "int", 32, 4, "float", 32, 4, runtime.GOARCH != "amd64" || archsimd.X86.AVX(), runtime.GOARCH != "amd64" || archsimd.X86.AVX(), func(x []int32, got []float32) {
		result := ConvertToFloat32Int32x4(archsimd.LoadInt32x4(x))
		result.Store(got)
	})
	check("ConvertToFloat32Uint32x4", "uint", 32, 4, "float", 32, 4, runtime.GOARCH != "amd64" || archsimd.X86.AVX(), runtime.GOARCH != "amd64" || archsimd.X86.AVX512(), func(x []uint32, got []float32) {
		result := ConvertToFloat32Uint32x4(archsimd.LoadUint32x4(x))
		result.Store(got)
	})
	check("ConvertToFloat64Int64x2", "int", 64, 2, "float", 64, 2, runtime.GOARCH != "amd64" || archsimd.X86.AVX(), runtime.GOARCH != "amd64" || archsimd.X86.AVX512(), func(x []int64, got []float64) {
		result := ConvertToFloat64Int64x2(archsimd.LoadInt64x2(x))
		result.Store(got)
	})
	check("ConvertToFloat64Uint64x2", "uint", 64, 2, "float", 64, 2, runtime.GOARCH != "amd64" || archsimd.X86.AVX(), runtime.GOARCH != "amd64" || archsimd.X86.AVX512(), func(x []uint64, got []float64) {
		result := ConvertToFloat64Uint64x2(archsimd.LoadUint64x2(x))
		result.Store(got)
	})
	check("ConvertToInt32Float32x4", "float", 32, 4, "int", 32, 4, runtime.GOARCH != "amd64" || archsimd.X86.AVX(), runtime.GOARCH != "amd64" || archsimd.X86.AVX(), func(x []float32, got []int32) {
		result := ConvertToInt32Float32x4(archsimd.LoadFloat32x4(x))
		result.Store(got)
	})
	check("ConvertToInt64Float64x2", "float", 64, 2, "int", 64, 2, runtime.GOARCH != "amd64" || archsimd.X86.AVX(), runtime.GOARCH != "amd64" || archsimd.X86.AVX512(), func(x []float64, got []int64) {
		result := ConvertToInt64Float64x2(archsimd.LoadFloat64x2(x))
		result.Store(got)
	})
	check("ConvertToUint32Float32x4", "float", 32, 4, "uint", 32, 4, runtime.GOARCH != "amd64" || archsimd.X86.AVX(), runtime.GOARCH != "amd64" || archsimd.X86.AVX512(), func(x []float32, got []uint32) {
		result := ConvertToUint32Float32x4(archsimd.LoadFloat32x4(x))
		result.Store(got)
	})
	check("ConvertToUint64Float64x2", "float", 64, 2, "uint", 64, 2, runtime.GOARCH != "amd64" || archsimd.X86.AVX(), runtime.GOARCH != "amd64" || archsimd.X86.AVX512(), func(x []float64, got []uint64) {
		result := ConvertToUint64Float64x2(archsimd.LoadFloat64x2(x))
		result.Store(got)
	})
}
