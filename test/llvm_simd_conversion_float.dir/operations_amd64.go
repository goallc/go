// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build amd64

package main

import (
	"simd/archsimd"
)

//go:noinline
func ConvertToFloat32Float64x4(x archsimd.Float64x4) archsimd.Float32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x4{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func ConvertToFloat32Float64x8(x archsimd.Float64x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x8{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func ConvertToFloat32Int32x8(x archsimd.Int32x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x8{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func ConvertToFloat32Int32x16(x archsimd.Int32x16) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func ConvertToFloat32Int64x2(x archsimd.Int64x2) archsimd.Float32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x4{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func ConvertToFloat32Int64x4(x archsimd.Int64x4) archsimd.Float32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x4{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func ConvertToFloat32Int64x8(x archsimd.Int64x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x8{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func ConvertToFloat32Uint32x8(x archsimd.Uint32x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x8{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func ConvertToFloat32Uint32x16(x archsimd.Uint32x16) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func ConvertToFloat32Uint64x2(x archsimd.Uint64x2) archsimd.Float32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x4{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func ConvertToFloat32Uint64x4(x archsimd.Uint64x4) archsimd.Float32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x4{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func ConvertToFloat32Uint64x8(x archsimd.Uint64x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x8{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func ConvertToFloat64Float32x4(x archsimd.Float32x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x4{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func ConvertToFloat64Float32x8(x archsimd.Float32x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func ConvertToFloat64Int32x4(x archsimd.Int32x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x4{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func ConvertToFloat64Int32x8(x archsimd.Int32x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func ConvertToFloat64Int64x4(x archsimd.Int64x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x4{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func ConvertToFloat64Int64x8(x archsimd.Int64x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func ConvertToFloat64Uint32x4(x archsimd.Uint32x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x4{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func ConvertToFloat64Uint32x8(x archsimd.Uint32x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func ConvertToFloat64Uint64x4(x archsimd.Uint64x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x4{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func ConvertToFloat64Uint64x8(x archsimd.Uint64x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func ConvertToInt32Float32x8(x archsimd.Float32x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x8{}
	}
	return x.ConvertToInt32()
}

//go:noinline
func ConvertToInt32Float32x16(x archsimd.Float32x16) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return x.ConvertToInt32()
}

//go:noinline
func ConvertToInt32Float64x2(x archsimd.Float64x2) archsimd.Int32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x4{}
	}
	return x.ConvertToInt32()
}

//go:noinline
func ConvertToInt32Float64x4(x archsimd.Float64x4) archsimd.Int32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x4{}
	}
	return x.ConvertToInt32()
}

//go:noinline
func ConvertToInt32Float64x8(x archsimd.Float64x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x8{}
	}
	return x.ConvertToInt32()
}

//go:noinline
func ConvertToInt64Float32x4(x archsimd.Float32x4) archsimd.Int64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x4{}
	}
	return x.ConvertToInt64()
}

//go:noinline
func ConvertToInt64Float32x8(x archsimd.Float32x8) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return x.ConvertToInt64()
}

//go:noinline
func ConvertToInt64Float64x4(x archsimd.Float64x4) archsimd.Int64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x4{}
	}
	return x.ConvertToInt64()
}

//go:noinline
func ConvertToInt64Float64x8(x archsimd.Float64x8) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return x.ConvertToInt64()
}

//go:noinline
func ConvertToUint32Float32x8(x archsimd.Float32x8) archsimd.Uint32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x8{}
	}
	return x.ConvertToUint32()
}

//go:noinline
func ConvertToUint32Float32x16(x archsimd.Float32x16) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return x.ConvertToUint32()
}

//go:noinline
func ConvertToUint32Float64x2(x archsimd.Float64x2) archsimd.Uint32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x4{}
	}
	return x.ConvertToUint32()
}

//go:noinline
func ConvertToUint32Float64x4(x archsimd.Float64x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x4{}
	}
	return x.ConvertToUint32()
}

//go:noinline
func ConvertToUint32Float64x8(x archsimd.Float64x8) archsimd.Uint32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x8{}
	}
	return x.ConvertToUint32()
}

//go:noinline
func ConvertToUint64Float32x4(x archsimd.Float32x4) archsimd.Uint64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x4{}
	}
	return x.ConvertToUint64()
}

//go:noinline
func ConvertToUint64Float32x8(x archsimd.Float32x8) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return x.ConvertToUint64()
}

//go:noinline
func ConvertToUint64Float64x4(x archsimd.Float64x4) archsimd.Uint64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x4{}
	}
	return x.ConvertToUint64()
}

//go:noinline
func ConvertToUint64Float64x8(x archsimd.Float64x8) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return x.ConvertToUint64()
}

func checkArch() {
	check("ConvertToFloat32Float64x4", "float", 64, 4, "float", 32, 4, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x []float64, got []float32) {
		result := ConvertToFloat32Float64x4(archsimd.LoadFloat64x4(x))
		result.Store(got)
	})
	check("ConvertToFloat32Float64x8", "float", 64, 8, "float", 32, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []float64, got []float32) {
		result := ConvertToFloat32Float64x8(archsimd.LoadFloat64x8(x))
		result.Store(got)
	})
	check("ConvertToFloat32Int32x8", "int", 32, 8, "float", 32, 8, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x []int32, got []float32) {
		result := ConvertToFloat32Int32x8(archsimd.LoadInt32x8(x))
		result.Store(got)
	})
	check("ConvertToFloat32Int32x16", "int", 32, 16, "float", 32, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []int32, got []float32) {
		result := ConvertToFloat32Int32x16(archsimd.LoadInt32x16(x))
		result.Store(got)
	})
	check("ConvertToFloat32Int64x2", "int", 64, 2, "float", 32, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x []int64, got []float32) {
		result := ConvertToFloat32Int64x2(archsimd.LoadInt64x2(x))
		result.Store(got)
	})
	check("ConvertToFloat32Int64x4", "int", 64, 4, "float", 32, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x []int64, got []float32) {
		result := ConvertToFloat32Int64x4(archsimd.LoadInt64x4(x))
		result.Store(got)
	})
	check("ConvertToFloat32Int64x8", "int", 64, 8, "float", 32, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []int64, got []float32) {
		result := ConvertToFloat32Int64x8(archsimd.LoadInt64x8(x))
		result.Store(got)
	})
	check("ConvertToFloat32Uint32x8", "uint", 32, 8, "float", 32, 8, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x []uint32, got []float32) {
		result := ConvertToFloat32Uint32x8(archsimd.LoadUint32x8(x))
		result.Store(got)
	})
	check("ConvertToFloat32Uint32x16", "uint", 32, 16, "float", 32, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []uint32, got []float32) {
		result := ConvertToFloat32Uint32x16(archsimd.LoadUint32x16(x))
		result.Store(got)
	})
	check("ConvertToFloat32Uint64x2", "uint", 64, 2, "float", 32, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x []uint64, got []float32) {
		result := ConvertToFloat32Uint64x2(archsimd.LoadUint64x2(x))
		result.Store(got)
	})
	check("ConvertToFloat32Uint64x4", "uint", 64, 4, "float", 32, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x []uint64, got []float32) {
		result := ConvertToFloat32Uint64x4(archsimd.LoadUint64x4(x))
		result.Store(got)
	})
	check("ConvertToFloat32Uint64x8", "uint", 64, 8, "float", 32, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []uint64, got []float32) {
		result := ConvertToFloat32Uint64x8(archsimd.LoadUint64x8(x))
		result.Store(got)
	})
	check("ConvertToFloat64Float32x4", "float", 32, 4, "float", 64, 4, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x []float32, got []float64) {
		result := ConvertToFloat64Float32x4(archsimd.LoadFloat32x4(x))
		result.Store(got)
	})
	check("ConvertToFloat64Float32x8", "float", 32, 8, "float", 64, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []float32, got []float64) {
		result := ConvertToFloat64Float32x8(archsimd.LoadFloat32x8(x))
		result.Store(got)
	})
	check("ConvertToFloat64Int32x4", "int", 32, 4, "float", 64, 4, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x []int32, got []float64) {
		result := ConvertToFloat64Int32x4(archsimd.LoadInt32x4(x))
		result.Store(got)
	})
	check("ConvertToFloat64Int32x8", "int", 32, 8, "float", 64, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []int32, got []float64) {
		result := ConvertToFloat64Int32x8(archsimd.LoadInt32x8(x))
		result.Store(got)
	})
	check("ConvertToFloat64Int64x4", "int", 64, 4, "float", 64, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x []int64, got []float64) {
		result := ConvertToFloat64Int64x4(archsimd.LoadInt64x4(x))
		result.Store(got)
	})
	check("ConvertToFloat64Int64x8", "int", 64, 8, "float", 64, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []int64, got []float64) {
		result := ConvertToFloat64Int64x8(archsimd.LoadInt64x8(x))
		result.Store(got)
	})
	check("ConvertToFloat64Uint32x4", "uint", 32, 4, "float", 64, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x []uint32, got []float64) {
		result := ConvertToFloat64Uint32x4(archsimd.LoadUint32x4(x))
		result.Store(got)
	})
	check("ConvertToFloat64Uint32x8", "uint", 32, 8, "float", 64, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []uint32, got []float64) {
		result := ConvertToFloat64Uint32x8(archsimd.LoadUint32x8(x))
		result.Store(got)
	})
	check("ConvertToFloat64Uint64x4", "uint", 64, 4, "float", 64, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x []uint64, got []float64) {
		result := ConvertToFloat64Uint64x4(archsimd.LoadUint64x4(x))
		result.Store(got)
	})
	check("ConvertToFloat64Uint64x8", "uint", 64, 8, "float", 64, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []uint64, got []float64) {
		result := ConvertToFloat64Uint64x8(archsimd.LoadUint64x8(x))
		result.Store(got)
	})
	check("ConvertToInt32Float32x8", "float", 32, 8, "int", 32, 8, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x []float32, got []int32) {
		result := ConvertToInt32Float32x8(archsimd.LoadFloat32x8(x))
		result.Store(got)
	})
	check("ConvertToInt32Float32x16", "float", 32, 16, "int", 32, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []float32, got []int32) {
		result := ConvertToInt32Float32x16(archsimd.LoadFloat32x16(x))
		result.Store(got)
	})
	check("ConvertToInt32Float64x2", "float", 64, 2, "int", 32, 4, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x []float64, got []int32) {
		result := ConvertToInt32Float64x2(archsimd.LoadFloat64x2(x))
		result.Store(got)
	})
	check("ConvertToInt32Float64x4", "float", 64, 4, "int", 32, 4, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x []float64, got []int32) {
		result := ConvertToInt32Float64x4(archsimd.LoadFloat64x4(x))
		result.Store(got)
	})
	check("ConvertToInt32Float64x8", "float", 64, 8, "int", 32, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []float64, got []int32) {
		result := ConvertToInt32Float64x8(archsimd.LoadFloat64x8(x))
		result.Store(got)
	})
	check("ConvertToInt64Float32x4", "float", 32, 4, "int", 64, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x []float32, got []int64) {
		result := ConvertToInt64Float32x4(archsimd.LoadFloat32x4(x))
		result.Store(got)
	})
	check("ConvertToInt64Float32x8", "float", 32, 8, "int", 64, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []float32, got []int64) {
		result := ConvertToInt64Float32x8(archsimd.LoadFloat32x8(x))
		result.Store(got)
	})
	check("ConvertToInt64Float64x4", "float", 64, 4, "int", 64, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x []float64, got []int64) {
		result := ConvertToInt64Float64x4(archsimd.LoadFloat64x4(x))
		result.Store(got)
	})
	check("ConvertToInt64Float64x8", "float", 64, 8, "int", 64, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []float64, got []int64) {
		result := ConvertToInt64Float64x8(archsimd.LoadFloat64x8(x))
		result.Store(got)
	})
	check("ConvertToUint32Float32x8", "float", 32, 8, "uint", 32, 8, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x []float32, got []uint32) {
		result := ConvertToUint32Float32x8(archsimd.LoadFloat32x8(x))
		result.Store(got)
	})
	check("ConvertToUint32Float32x16", "float", 32, 16, "uint", 32, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []float32, got []uint32) {
		result := ConvertToUint32Float32x16(archsimd.LoadFloat32x16(x))
		result.Store(got)
	})
	check("ConvertToUint32Float64x2", "float", 64, 2, "uint", 32, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x []float64, got []uint32) {
		result := ConvertToUint32Float64x2(archsimd.LoadFloat64x2(x))
		result.Store(got)
	})
	check("ConvertToUint32Float64x4", "float", 64, 4, "uint", 32, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x []float64, got []uint32) {
		result := ConvertToUint32Float64x4(archsimd.LoadFloat64x4(x))
		result.Store(got)
	})
	check("ConvertToUint32Float64x8", "float", 64, 8, "uint", 32, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []float64, got []uint32) {
		result := ConvertToUint32Float64x8(archsimd.LoadFloat64x8(x))
		result.Store(got)
	})
	check("ConvertToUint64Float32x4", "float", 32, 4, "uint", 64, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x []float32, got []uint64) {
		result := ConvertToUint64Float32x4(archsimd.LoadFloat32x4(x))
		result.Store(got)
	})
	check("ConvertToUint64Float32x8", "float", 32, 8, "uint", 64, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []float32, got []uint64) {
		result := ConvertToUint64Float32x8(archsimd.LoadFloat32x8(x))
		result.Store(got)
	})
	check("ConvertToUint64Float64x4", "float", 64, 4, "uint", 64, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x []float64, got []uint64) {
		result := ConvertToUint64Float64x4(archsimd.LoadFloat64x4(x))
		result.Store(got)
	})
	check("ConvertToUint64Float64x8", "float", 64, 8, "uint", 64, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x []float64, got []uint64) {
		result := ConvertToUint64Float64x8(archsimd.LoadFloat64x8(x))
		result.Store(got)
	})
}
