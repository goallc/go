// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func dynamicConcatPermuteInt8x16(x, y archsimd.Int8x16, indices archsimd.Uint8x16) archsimd.Int8x16 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Int8x16{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteUint8x16(x, y archsimd.Uint8x16, indices archsimd.Uint8x16) archsimd.Uint8x16 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Uint8x16{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteInt8x32(x, y archsimd.Int8x32, indices archsimd.Uint8x32) archsimd.Int8x32 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Int8x32{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteUint8x32(x, y archsimd.Uint8x32, indices archsimd.Uint8x32) archsimd.Uint8x32 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Uint8x32{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteInt8x64(x, y archsimd.Int8x64, indices archsimd.Uint8x64) archsimd.Int8x64 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Int8x64{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteUint8x64(x, y archsimd.Uint8x64, indices archsimd.Uint8x64) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Uint8x64{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteInt16x8(x, y archsimd.Int16x8, indices archsimd.Uint16x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x8{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteUint16x8(x, y archsimd.Uint16x8, indices archsimd.Uint16x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x8{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteInt16x16(x, y archsimd.Int16x16, indices archsimd.Uint16x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x16{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteUint16x16(x, y archsimd.Uint16x16, indices archsimd.Uint16x16) archsimd.Uint16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x16{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteInt16x32(x, y archsimd.Int16x32, indices archsimd.Uint16x32) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteUint16x32(x, y archsimd.Uint16x32, indices archsimd.Uint16x32) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteFloat32x4(x, y archsimd.Float32x4, indices archsimd.Uint32x4) archsimd.Float32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x4{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteInt32x4(x, y archsimd.Int32x4, indices archsimd.Uint32x4) archsimd.Int32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x4{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteUint32x4(x, y archsimd.Uint32x4, indices archsimd.Uint32x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x4{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteFloat32x8(x, y archsimd.Float32x8, indices archsimd.Uint32x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x8{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteInt32x8(x, y archsimd.Int32x8, indices archsimd.Uint32x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x8{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteUint32x8(x, y archsimd.Uint32x8, indices archsimd.Uint32x8) archsimd.Uint32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x8{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteFloat32x16(x, y archsimd.Float32x16, indices archsimd.Uint32x16) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteInt32x16(x, y archsimd.Int32x16, indices archsimd.Uint32x16) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteUint32x16(x, y archsimd.Uint32x16, indices archsimd.Uint32x16) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteFloat64x2(x, y archsimd.Float64x2, indices archsimd.Uint64x2) archsimd.Float64x2 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x2{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteInt64x2(x, y archsimd.Int64x2, indices archsimd.Uint64x2) archsimd.Int64x2 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x2{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteUint64x2(x, y archsimd.Uint64x2, indices archsimd.Uint64x2) archsimd.Uint64x2 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x2{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteFloat64x4(x, y archsimd.Float64x4, indices archsimd.Uint64x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x4{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteInt64x4(x, y archsimd.Int64x4, indices archsimd.Uint64x4) archsimd.Int64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x4{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteUint64x4(x, y archsimd.Uint64x4, indices archsimd.Uint64x4) archsimd.Uint64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x4{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteFloat64x8(x, y archsimd.Float64x8, indices archsimd.Uint64x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteInt64x8(x, y archsimd.Int64x8, indices archsimd.Uint64x8) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicConcatPermuteUint64x8(x, y archsimd.Uint64x8, indices archsimd.Uint64x8) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func dynamicPermuteInt8x16(x, y archsimd.Int8x16, indices archsimd.Uint8x16) archsimd.Int8x16 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Int8x16{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteUint8x16(x, y archsimd.Uint8x16, indices archsimd.Uint8x16) archsimd.Uint8x16 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Uint8x16{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteInt8x32(x, y archsimd.Int8x32, indices archsimd.Uint8x32) archsimd.Int8x32 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Int8x32{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteUint8x32(x, y archsimd.Uint8x32, indices archsimd.Uint8x32) archsimd.Uint8x32 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Uint8x32{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteInt8x64(x, y archsimd.Int8x64, indices archsimd.Uint8x64) archsimd.Int8x64 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Int8x64{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteUint8x64(x, y archsimd.Uint8x64, indices archsimd.Uint8x64) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Uint8x64{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteInt16x8(x, y archsimd.Int16x8, indices archsimd.Uint16x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x8{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteUint16x8(x, y archsimd.Uint16x8, indices archsimd.Uint16x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x8{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteInt16x16(x, y archsimd.Int16x16, indices archsimd.Uint16x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x16{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteUint16x16(x, y archsimd.Uint16x16, indices archsimd.Uint16x16) archsimd.Uint16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x16{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteInt16x32(x, y archsimd.Int16x32, indices archsimd.Uint16x32) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteUint16x32(x, y archsimd.Uint16x32, indices archsimd.Uint16x32) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteFloat32x8(x, y archsimd.Float32x8, indices archsimd.Uint32x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Float32x8{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteInt32x8(x, y archsimd.Int32x8, indices archsimd.Uint32x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteUint32x8(x, y archsimd.Uint32x8, indices archsimd.Uint32x8) archsimd.Uint32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x8{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteFloat32x16(x, y archsimd.Float32x16, indices archsimd.Uint32x16) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteInt32x16(x, y archsimd.Int32x16, indices archsimd.Uint32x16) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteUint32x16(x, y archsimd.Uint32x16, indices archsimd.Uint32x16) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteFloat64x4(x, y archsimd.Float64x4, indices archsimd.Uint64x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x4{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteInt64x4(x, y archsimd.Int64x4, indices archsimd.Uint64x4) archsimd.Int64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x4{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteUint64x4(x, y archsimd.Uint64x4, indices archsimd.Uint64x4) archsimd.Uint64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x4{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteFloat64x8(x, y archsimd.Float64x8, indices archsimd.Uint64x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteInt64x8(x, y archsimd.Int64x8, indices archsimd.Uint64x8) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteUint64x8(x, y archsimd.Uint64x8, indices archsimd.Uint64x8) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return x.Permute(indices)
}

//go:noinline
func dynamicPermuteOrZeroInt8x16(x, y archsimd.Int8x16, indices archsimd.Int8x16) archsimd.Int8x16 {
	if !archsimd.X86.AVX() {
		return archsimd.Int8x16{}
	}
	return x.PermuteOrZero(indices)
}

//go:noinline
func dynamicPermuteOrZeroUint8x16(x, y archsimd.Uint8x16, indices archsimd.Int8x16) archsimd.Uint8x16 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint8x16{}
	}
	return x.PermuteOrZero(indices)
}

//go:noinline
func dynamicPermuteOrZeroGroupedInt8x32(x, y archsimd.Int8x32, indices archsimd.Int8x32) archsimd.Int8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int8x32{}
	}
	return x.PermuteOrZeroGrouped(indices)
}

//go:noinline
func dynamicPermuteOrZeroGroupedInt8x64(x, y archsimd.Int8x64, indices archsimd.Int8x64) archsimd.Int8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x64{}
	}
	return x.PermuteOrZeroGrouped(indices)
}

//go:noinline
func dynamicPermuteOrZeroGroupedUint8x32(x, y archsimd.Uint8x32, indices archsimd.Int8x32) archsimd.Uint8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x32{}
	}
	return x.PermuteOrZeroGrouped(indices)
}

//go:noinline
func dynamicPermuteOrZeroGroupedUint8x64(x, y archsimd.Uint8x64, indices archsimd.Int8x64) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x64{}
	}
	return x.PermuteOrZeroGrouped(indices)
}

func checkArch() {
	checkDynamic[int8, uint8]("ConcatPermuteInt8x16", "ConcatPermute", 8, 16, true, archsimd.X86.AVX512VBMI(), func(x, y []int8, indices []uint8, got []int8) {
		z := dynamicConcatPermuteInt8x16(archsimd.LoadInt8x16(x), archsimd.LoadInt8x16(y), archsimd.LoadUint8x16(indices))
		z.Store(got)
	})
	checkDynamic[uint8, uint8]("ConcatPermuteUint8x16", "ConcatPermute", 8, 16, true, archsimd.X86.AVX512VBMI(), func(x, y []uint8, indices []uint8, got []uint8) {
		z := dynamicConcatPermuteUint8x16(archsimd.LoadUint8x16(x), archsimd.LoadUint8x16(y), archsimd.LoadUint8x16(indices))
		z.Store(got)
	})
	checkDynamic[int8, uint8]("ConcatPermuteInt8x32", "ConcatPermute", 8, 32, archsimd.X86.AVX(), archsimd.X86.AVX512VBMI(), func(x, y []int8, indices []uint8, got []int8) {
		z := dynamicConcatPermuteInt8x32(archsimd.LoadInt8x32(x), archsimd.LoadInt8x32(y), archsimd.LoadUint8x32(indices))
		z.Store(got)
	})
	checkDynamic[uint8, uint8]("ConcatPermuteUint8x32", "ConcatPermute", 8, 32, archsimd.X86.AVX(), archsimd.X86.AVX512VBMI(), func(x, y []uint8, indices []uint8, got []uint8) {
		z := dynamicConcatPermuteUint8x32(archsimd.LoadUint8x32(x), archsimd.LoadUint8x32(y), archsimd.LoadUint8x32(indices))
		z.Store(got)
	})
	checkDynamic[int8, uint8]("ConcatPermuteInt8x64", "ConcatPermute", 8, 64, archsimd.X86.AVX512(), archsimd.X86.AVX512VBMI(), func(x, y []int8, indices []uint8, got []int8) {
		z := dynamicConcatPermuteInt8x64(archsimd.LoadInt8x64(x), archsimd.LoadInt8x64(y), archsimd.LoadUint8x64(indices))
		z.Store(got)
	})
	checkDynamic[uint8, uint8]("ConcatPermuteUint8x64", "ConcatPermute", 8, 64, archsimd.X86.AVX512(), archsimd.X86.AVX512VBMI(), func(x, y []uint8, indices []uint8, got []uint8) {
		z := dynamicConcatPermuteUint8x64(archsimd.LoadUint8x64(x), archsimd.LoadUint8x64(y), archsimd.LoadUint8x64(indices))
		z.Store(got)
	})
	checkDynamic[int16, uint16]("ConcatPermuteInt16x8", "ConcatPermute", 16, 8, true, archsimd.X86.AVX512(), func(x, y []int16, indices []uint16, got []int16) {
		z := dynamicConcatPermuteInt16x8(archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y), archsimd.LoadUint16x8(indices))
		z.Store(got)
	})
	checkDynamic[uint16, uint16]("ConcatPermuteUint16x8", "ConcatPermute", 16, 8, true, archsimd.X86.AVX512(), func(x, y []uint16, indices []uint16, got []uint16) {
		z := dynamicConcatPermuteUint16x8(archsimd.LoadUint16x8(x), archsimd.LoadUint16x8(y), archsimd.LoadUint16x8(indices))
		z.Store(got)
	})
	checkDynamic[int16, uint16]("ConcatPermuteInt16x16", "ConcatPermute", 16, 16, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []int16, indices []uint16, got []int16) {
		z := dynamicConcatPermuteInt16x16(archsimd.LoadInt16x16(x), archsimd.LoadInt16x16(y), archsimd.LoadUint16x16(indices))
		z.Store(got)
	})
	checkDynamic[uint16, uint16]("ConcatPermuteUint16x16", "ConcatPermute", 16, 16, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []uint16, indices []uint16, got []uint16) {
		z := dynamicConcatPermuteUint16x16(archsimd.LoadUint16x16(x), archsimd.LoadUint16x16(y), archsimd.LoadUint16x16(indices))
		z.Store(got)
	})
	checkDynamic[int16, uint16]("ConcatPermuteInt16x32", "ConcatPermute", 16, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []int16, indices []uint16, got []int16) {
		z := dynamicConcatPermuteInt16x32(archsimd.LoadInt16x32(x), archsimd.LoadInt16x32(y), archsimd.LoadUint16x32(indices))
		z.Store(got)
	})
	checkDynamic[uint16, uint16]("ConcatPermuteUint16x32", "ConcatPermute", 16, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []uint16, indices []uint16, got []uint16) {
		z := dynamicConcatPermuteUint16x32(archsimd.LoadUint16x32(x), archsimd.LoadUint16x32(y), archsimd.LoadUint16x32(indices))
		z.Store(got)
	})
	checkDynamic[float32, uint32]("ConcatPermuteFloat32x4", "ConcatPermute", 32, 4, true, archsimd.X86.AVX512(), func(x, y []float32, indices []uint32, got []float32) {
		z := dynamicConcatPermuteFloat32x4(archsimd.LoadFloat32x4(x), archsimd.LoadFloat32x4(y), archsimd.LoadUint32x4(indices))
		z.Store(got)
	})
	checkDynamic[int32, uint32]("ConcatPermuteInt32x4", "ConcatPermute", 32, 4, true, archsimd.X86.AVX512(), func(x, y []int32, indices []uint32, got []int32) {
		z := dynamicConcatPermuteInt32x4(archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y), archsimd.LoadUint32x4(indices))
		z.Store(got)
	})
	checkDynamic[uint32, uint32]("ConcatPermuteUint32x4", "ConcatPermute", 32, 4, true, archsimd.X86.AVX512(), func(x, y []uint32, indices []uint32, got []uint32) {
		z := dynamicConcatPermuteUint32x4(archsimd.LoadUint32x4(x), archsimd.LoadUint32x4(y), archsimd.LoadUint32x4(indices))
		z.Store(got)
	})
	checkDynamic[float32, uint32]("ConcatPermuteFloat32x8", "ConcatPermute", 32, 8, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []float32, indices []uint32, got []float32) {
		z := dynamicConcatPermuteFloat32x8(archsimd.LoadFloat32x8(x), archsimd.LoadFloat32x8(y), archsimd.LoadUint32x8(indices))
		z.Store(got)
	})
	checkDynamic[int32, uint32]("ConcatPermuteInt32x8", "ConcatPermute", 32, 8, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []int32, indices []uint32, got []int32) {
		z := dynamicConcatPermuteInt32x8(archsimd.LoadInt32x8(x), archsimd.LoadInt32x8(y), archsimd.LoadUint32x8(indices))
		z.Store(got)
	})
	checkDynamic[uint32, uint32]("ConcatPermuteUint32x8", "ConcatPermute", 32, 8, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []uint32, indices []uint32, got []uint32) {
		z := dynamicConcatPermuteUint32x8(archsimd.LoadUint32x8(x), archsimd.LoadUint32x8(y), archsimd.LoadUint32x8(indices))
		z.Store(got)
	})
	checkDynamic[float32, uint32]("ConcatPermuteFloat32x16", "ConcatPermute", 32, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []float32, indices []uint32, got []float32) {
		z := dynamicConcatPermuteFloat32x16(archsimd.LoadFloat32x16(x), archsimd.LoadFloat32x16(y), archsimd.LoadUint32x16(indices))
		z.Store(got)
	})
	checkDynamic[int32, uint32]("ConcatPermuteInt32x16", "ConcatPermute", 32, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []int32, indices []uint32, got []int32) {
		z := dynamicConcatPermuteInt32x16(archsimd.LoadInt32x16(x), archsimd.LoadInt32x16(y), archsimd.LoadUint32x16(indices))
		z.Store(got)
	})
	checkDynamic[uint32, uint32]("ConcatPermuteUint32x16", "ConcatPermute", 32, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []uint32, indices []uint32, got []uint32) {
		z := dynamicConcatPermuteUint32x16(archsimd.LoadUint32x16(x), archsimd.LoadUint32x16(y), archsimd.LoadUint32x16(indices))
		z.Store(got)
	})
	checkDynamic[float64, uint64]("ConcatPermuteFloat64x2", "ConcatPermute", 64, 2, true, archsimd.X86.AVX512(), func(x, y []float64, indices []uint64, got []float64) {
		z := dynamicConcatPermuteFloat64x2(archsimd.LoadFloat64x2(x), archsimd.LoadFloat64x2(y), archsimd.LoadUint64x2(indices))
		z.Store(got)
	})
	checkDynamic[int64, uint64]("ConcatPermuteInt64x2", "ConcatPermute", 64, 2, true, archsimd.X86.AVX512(), func(x, y []int64, indices []uint64, got []int64) {
		z := dynamicConcatPermuteInt64x2(archsimd.LoadInt64x2(x), archsimd.LoadInt64x2(y), archsimd.LoadUint64x2(indices))
		z.Store(got)
	})
	checkDynamic[uint64, uint64]("ConcatPermuteUint64x2", "ConcatPermute", 64, 2, true, archsimd.X86.AVX512(), func(x, y []uint64, indices []uint64, got []uint64) {
		z := dynamicConcatPermuteUint64x2(archsimd.LoadUint64x2(x), archsimd.LoadUint64x2(y), archsimd.LoadUint64x2(indices))
		z.Store(got)
	})
	checkDynamic[float64, uint64]("ConcatPermuteFloat64x4", "ConcatPermute", 64, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []float64, indices []uint64, got []float64) {
		z := dynamicConcatPermuteFloat64x4(archsimd.LoadFloat64x4(x), archsimd.LoadFloat64x4(y), archsimd.LoadUint64x4(indices))
		z.Store(got)
	})
	checkDynamic[int64, uint64]("ConcatPermuteInt64x4", "ConcatPermute", 64, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []int64, indices []uint64, got []int64) {
		z := dynamicConcatPermuteInt64x4(archsimd.LoadInt64x4(x), archsimd.LoadInt64x4(y), archsimd.LoadUint64x4(indices))
		z.Store(got)
	})
	checkDynamic[uint64, uint64]("ConcatPermuteUint64x4", "ConcatPermute", 64, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []uint64, indices []uint64, got []uint64) {
		z := dynamicConcatPermuteUint64x4(archsimd.LoadUint64x4(x), archsimd.LoadUint64x4(y), archsimd.LoadUint64x4(indices))
		z.Store(got)
	})
	checkDynamic[float64, uint64]("ConcatPermuteFloat64x8", "ConcatPermute", 64, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []float64, indices []uint64, got []float64) {
		z := dynamicConcatPermuteFloat64x8(archsimd.LoadFloat64x8(x), archsimd.LoadFloat64x8(y), archsimd.LoadUint64x8(indices))
		z.Store(got)
	})
	checkDynamic[int64, uint64]("ConcatPermuteInt64x8", "ConcatPermute", 64, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []int64, indices []uint64, got []int64) {
		z := dynamicConcatPermuteInt64x8(archsimd.LoadInt64x8(x), archsimd.LoadInt64x8(y), archsimd.LoadUint64x8(indices))
		z.Store(got)
	})
	checkDynamic[uint64, uint64]("ConcatPermuteUint64x8", "ConcatPermute", 64, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []uint64, indices []uint64, got []uint64) {
		z := dynamicConcatPermuteUint64x8(archsimd.LoadUint64x8(x), archsimd.LoadUint64x8(y), archsimd.LoadUint64x8(indices))
		z.Store(got)
	})
	checkDynamic[int8, uint8]("PermuteInt8x16", "Permute", 8, 16, true, archsimd.X86.AVX512VBMI(), func(x, y []int8, indices []uint8, got []int8) {
		z := dynamicPermuteInt8x16(archsimd.LoadInt8x16(x), archsimd.LoadInt8x16(y), archsimd.LoadUint8x16(indices))
		z.Store(got)
	})
	checkDynamic[uint8, uint8]("PermuteUint8x16", "Permute", 8, 16, true, archsimd.X86.AVX512VBMI(), func(x, y []uint8, indices []uint8, got []uint8) {
		z := dynamicPermuteUint8x16(archsimd.LoadUint8x16(x), archsimd.LoadUint8x16(y), archsimd.LoadUint8x16(indices))
		z.Store(got)
	})
	checkDynamic[int8, uint8]("PermuteInt8x32", "Permute", 8, 32, archsimd.X86.AVX(), archsimd.X86.AVX512VBMI(), func(x, y []int8, indices []uint8, got []int8) {
		z := dynamicPermuteInt8x32(archsimd.LoadInt8x32(x), archsimd.LoadInt8x32(y), archsimd.LoadUint8x32(indices))
		z.Store(got)
	})
	checkDynamic[uint8, uint8]("PermuteUint8x32", "Permute", 8, 32, archsimd.X86.AVX(), archsimd.X86.AVX512VBMI(), func(x, y []uint8, indices []uint8, got []uint8) {
		z := dynamicPermuteUint8x32(archsimd.LoadUint8x32(x), archsimd.LoadUint8x32(y), archsimd.LoadUint8x32(indices))
		z.Store(got)
	})
	checkDynamic[int8, uint8]("PermuteInt8x64", "Permute", 8, 64, archsimd.X86.AVX512(), archsimd.X86.AVX512VBMI(), func(x, y []int8, indices []uint8, got []int8) {
		z := dynamicPermuteInt8x64(archsimd.LoadInt8x64(x), archsimd.LoadInt8x64(y), archsimd.LoadUint8x64(indices))
		z.Store(got)
	})
	checkDynamic[uint8, uint8]("PermuteUint8x64", "Permute", 8, 64, archsimd.X86.AVX512(), archsimd.X86.AVX512VBMI(), func(x, y []uint8, indices []uint8, got []uint8) {
		z := dynamicPermuteUint8x64(archsimd.LoadUint8x64(x), archsimd.LoadUint8x64(y), archsimd.LoadUint8x64(indices))
		z.Store(got)
	})
	checkDynamic[int16, uint16]("PermuteInt16x8", "Permute", 16, 8, true, archsimd.X86.AVX512(), func(x, y []int16, indices []uint16, got []int16) {
		z := dynamicPermuteInt16x8(archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y), archsimd.LoadUint16x8(indices))
		z.Store(got)
	})
	checkDynamic[uint16, uint16]("PermuteUint16x8", "Permute", 16, 8, true, archsimd.X86.AVX512(), func(x, y []uint16, indices []uint16, got []uint16) {
		z := dynamicPermuteUint16x8(archsimd.LoadUint16x8(x), archsimd.LoadUint16x8(y), archsimd.LoadUint16x8(indices))
		z.Store(got)
	})
	checkDynamic[int16, uint16]("PermuteInt16x16", "Permute", 16, 16, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []int16, indices []uint16, got []int16) {
		z := dynamicPermuteInt16x16(archsimd.LoadInt16x16(x), archsimd.LoadInt16x16(y), archsimd.LoadUint16x16(indices))
		z.Store(got)
	})
	checkDynamic[uint16, uint16]("PermuteUint16x16", "Permute", 16, 16, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []uint16, indices []uint16, got []uint16) {
		z := dynamicPermuteUint16x16(archsimd.LoadUint16x16(x), archsimd.LoadUint16x16(y), archsimd.LoadUint16x16(indices))
		z.Store(got)
	})
	checkDynamic[int16, uint16]("PermuteInt16x32", "Permute", 16, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []int16, indices []uint16, got []int16) {
		z := dynamicPermuteInt16x32(archsimd.LoadInt16x32(x), archsimd.LoadInt16x32(y), archsimd.LoadUint16x32(indices))
		z.Store(got)
	})
	checkDynamic[uint16, uint16]("PermuteUint16x32", "Permute", 16, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []uint16, indices []uint16, got []uint16) {
		z := dynamicPermuteUint16x32(archsimd.LoadUint16x32(x), archsimd.LoadUint16x32(y), archsimd.LoadUint16x32(indices))
		z.Store(got)
	})
	checkDynamic[float32, uint32]("PermuteFloat32x8", "Permute", 32, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y []float32, indices []uint32, got []float32) {
		z := dynamicPermuteFloat32x8(archsimd.LoadFloat32x8(x), archsimd.LoadFloat32x8(y), archsimd.LoadUint32x8(indices))
		z.Store(got)
	})
	checkDynamic[int32, uint32]("PermuteInt32x8", "Permute", 32, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y []int32, indices []uint32, got []int32) {
		z := dynamicPermuteInt32x8(archsimd.LoadInt32x8(x), archsimd.LoadInt32x8(y), archsimd.LoadUint32x8(indices))
		z.Store(got)
	})
	checkDynamic[uint32, uint32]("PermuteUint32x8", "Permute", 32, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y []uint32, indices []uint32, got []uint32) {
		z := dynamicPermuteUint32x8(archsimd.LoadUint32x8(x), archsimd.LoadUint32x8(y), archsimd.LoadUint32x8(indices))
		z.Store(got)
	})
	checkDynamic[float32, uint32]("PermuteFloat32x16", "Permute", 32, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []float32, indices []uint32, got []float32) {
		z := dynamicPermuteFloat32x16(archsimd.LoadFloat32x16(x), archsimd.LoadFloat32x16(y), archsimd.LoadUint32x16(indices))
		z.Store(got)
	})
	checkDynamic[int32, uint32]("PermuteInt32x16", "Permute", 32, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []int32, indices []uint32, got []int32) {
		z := dynamicPermuteInt32x16(archsimd.LoadInt32x16(x), archsimd.LoadInt32x16(y), archsimd.LoadUint32x16(indices))
		z.Store(got)
	})
	checkDynamic[uint32, uint32]("PermuteUint32x16", "Permute", 32, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []uint32, indices []uint32, got []uint32) {
		z := dynamicPermuteUint32x16(archsimd.LoadUint32x16(x), archsimd.LoadUint32x16(y), archsimd.LoadUint32x16(indices))
		z.Store(got)
	})
	checkDynamic[float64, uint64]("PermuteFloat64x4", "Permute", 64, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []float64, indices []uint64, got []float64) {
		z := dynamicPermuteFloat64x4(archsimd.LoadFloat64x4(x), archsimd.LoadFloat64x4(y), archsimd.LoadUint64x4(indices))
		z.Store(got)
	})
	checkDynamic[int64, uint64]("PermuteInt64x4", "Permute", 64, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []int64, indices []uint64, got []int64) {
		z := dynamicPermuteInt64x4(archsimd.LoadInt64x4(x), archsimd.LoadInt64x4(y), archsimd.LoadUint64x4(indices))
		z.Store(got)
	})
	checkDynamic[uint64, uint64]("PermuteUint64x4", "Permute", 64, 4, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []uint64, indices []uint64, got []uint64) {
		z := dynamicPermuteUint64x4(archsimd.LoadUint64x4(x), archsimd.LoadUint64x4(y), archsimd.LoadUint64x4(indices))
		z.Store(got)
	})
	checkDynamic[float64, uint64]("PermuteFloat64x8", "Permute", 64, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []float64, indices []uint64, got []float64) {
		z := dynamicPermuteFloat64x8(archsimd.LoadFloat64x8(x), archsimd.LoadFloat64x8(y), archsimd.LoadUint64x8(indices))
		z.Store(got)
	})
	checkDynamic[int64, uint64]("PermuteInt64x8", "Permute", 64, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []int64, indices []uint64, got []int64) {
		z := dynamicPermuteInt64x8(archsimd.LoadInt64x8(x), archsimd.LoadInt64x8(y), archsimd.LoadUint64x8(indices))
		z.Store(got)
	})
	checkDynamic[uint64, uint64]("PermuteUint64x8", "Permute", 64, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []uint64, indices []uint64, got []uint64) {
		z := dynamicPermuteUint64x8(archsimd.LoadUint64x8(x), archsimd.LoadUint64x8(y), archsimd.LoadUint64x8(indices))
		z.Store(got)
	})
	checkDynamic[int8, int8]("PermuteOrZeroInt8x16", "PermuteOrZero", 8, 16, true, archsimd.X86.AVX(), func(x, y []int8, indices []int8, got []int8) {
		z := dynamicPermuteOrZeroInt8x16(archsimd.LoadInt8x16(x), archsimd.LoadInt8x16(y), archsimd.LoadInt8x16(indices))
		z.Store(got)
	})
	checkDynamic[uint8, int8]("PermuteOrZeroUint8x16", "PermuteOrZero", 8, 16, true, archsimd.X86.AVX(), func(x, y []uint8, indices []int8, got []uint8) {
		z := dynamicPermuteOrZeroUint8x16(archsimd.LoadUint8x16(x), archsimd.LoadUint8x16(y), archsimd.LoadInt8x16(indices))
		z.Store(got)
	})
	checkDynamic[int8, int8]("PermuteOrZeroGroupedInt8x32", "PermuteOrZeroGrouped", 8, 32, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y []int8, indices []int8, got []int8) {
		z := dynamicPermuteOrZeroGroupedInt8x32(archsimd.LoadInt8x32(x), archsimd.LoadInt8x32(y), archsimd.LoadInt8x32(indices))
		z.Store(got)
	})
	checkDynamic[int8, int8]("PermuteOrZeroGroupedInt8x64", "PermuteOrZeroGrouped", 8, 64, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []int8, indices []int8, got []int8) {
		z := dynamicPermuteOrZeroGroupedInt8x64(archsimd.LoadInt8x64(x), archsimd.LoadInt8x64(y), archsimd.LoadInt8x64(indices))
		z.Store(got)
	})
	checkDynamic[uint8, int8]("PermuteOrZeroGroupedUint8x32", "PermuteOrZeroGrouped", 8, 32, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y []uint8, indices []int8, got []uint8) {
		z := dynamicPermuteOrZeroGroupedUint8x32(archsimd.LoadUint8x32(x), archsimd.LoadUint8x32(y), archsimd.LoadInt8x32(indices))
		z.Store(got)
	})
	checkDynamic[uint8, int8]("PermuteOrZeroGroupedUint8x64", "PermuteOrZeroGrouped", 8, 64, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []uint8, indices []int8, got []uint8) {
		z := dynamicPermuteOrZeroGroupedUint8x64(archsimd.LoadUint8x64(x), archsimd.LoadUint8x64(y), archsimd.LoadInt8x64(indices))
		z.Store(got)
	})
}
