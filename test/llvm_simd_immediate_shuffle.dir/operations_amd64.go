// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func immediateConcatPermute128ScalarsFloat32x8(x, y archsimd.Float32x8, c uint16) archsimd.Float32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x8{}
	}
	return x.ConcatPermute128Scalars(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateConcatPermute128ScalarsFloat64x4(x, y archsimd.Float64x4, c uint16) archsimd.Float64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x4{}
	}
	return x.ConcatPermute128Scalars(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateConcatPermute128ScalarsInt8x32(x, y archsimd.Int8x32, c uint16) archsimd.Int8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int8x32{}
	}
	return x.ConcatPermute128Scalars(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateConcatPermute128ScalarsInt16x16(x, y archsimd.Int16x16, c uint16) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return x.ConcatPermute128Scalars(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateConcatPermute128ScalarsInt32x8(x, y archsimd.Int32x8, c uint16) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return x.ConcatPermute128Scalars(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateConcatPermute128ScalarsInt64x4(x, y archsimd.Int64x4, c uint16) archsimd.Int64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x4{}
	}
	return x.ConcatPermute128Scalars(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateConcatPermute128ScalarsUint8x32(x, y archsimd.Uint8x32, c uint16) archsimd.Uint8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x32{}
	}
	return x.ConcatPermute128Scalars(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateConcatPermute128ScalarsUint16x16(x, y archsimd.Uint16x16, c uint16) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return x.ConcatPermute128Scalars(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateConcatPermute128ScalarsUint32x8(x, y archsimd.Uint32x8, c uint16) archsimd.Uint32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x8{}
	}
	return x.ConcatPermute128Scalars(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateConcatPermute128ScalarsUint64x4(x, y archsimd.Uint64x4, c uint16) archsimd.Uint64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x4{}
	}
	return x.ConcatPermute128Scalars(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateConcatShiftBytesRightGroupedUint8x32(x, y archsimd.Uint8x32, c uint16) archsimd.Uint8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x32{}
	}
	return x.ConcatShiftBytesRightGrouped(y, uint64(c))
}

//go:noinline
func immediateConcatShiftBytesRightGroupedUint8x64(x, y archsimd.Uint8x64, c uint16) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x64{}
	}
	return x.ConcatShiftBytesRightGrouped(y, uint64(c))
}

//go:noinline
func immediateConcatShiftBytesRightUint8x16(x, y archsimd.Uint8x16, c uint16) archsimd.Uint8x16 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint8x16{}
	}
	return x.ConcatShiftBytesRight(y, uint64(c))
}

//go:noinline
func immediateconcatSelectedConstantFloat32x4(x, y archsimd.Float32x4, c uint16) archsimd.Float32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x4{}
	}
	return x.ConcatPermuteScalars(uint8(c)&7, uint8(c>>3)&7, uint8(c>>6)&7, uint8(c>>9)&7, y)
}

//go:noinline
func immediateconcatSelectedConstantFloat64x2(x, y archsimd.Float64x2, c uint16) archsimd.Float64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x2{}
	}
	return x.ConcatPermuteScalars(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateconcatSelectedConstantGroupedFloat32x8(x, y archsimd.Float32x8, c uint16) archsimd.Float32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x8{}
	}
	return x.ConcatPermuteScalarsGrouped(uint8(c)&7, uint8(c>>3)&7, uint8(c>>6)&7, uint8(c>>9)&7, y)
}

//go:noinline
func immediateconcatSelectedConstantGroupedFloat32x16(x, y archsimd.Float32x16, c uint16) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return x.ConcatPermuteScalarsGrouped(uint8(c)&7, uint8(c>>3)&7, uint8(c>>6)&7, uint8(c>>9)&7, y)
}

//go:noinline
func immediateconcatSelectedConstantGroupedFloat64x4(x, y archsimd.Float64x4, c uint16) archsimd.Float64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x4{}
	}
	return x.ConcatPermuteScalarsGrouped(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateconcatSelectedConstantGroupedFloat64x8(x, y archsimd.Float64x8, c uint16) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.ConcatPermuteScalarsGrouped(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateconcatSelectedConstantGroupedInt32x8(x, y archsimd.Int32x8, c uint16) archsimd.Int32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x8{}
	}
	return x.ConcatPermuteScalarsGrouped(uint8(c)&7, uint8(c>>3)&7, uint8(c>>6)&7, uint8(c>>9)&7, y)
}

//go:noinline
func immediateconcatSelectedConstantGroupedInt32x16(x, y archsimd.Int32x16, c uint16) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return x.ConcatPermuteScalarsGrouped(uint8(c)&7, uint8(c>>3)&7, uint8(c>>6)&7, uint8(c>>9)&7, y)
}

//go:noinline
func immediateconcatSelectedConstantGroupedInt64x4(x, y archsimd.Int64x4, c uint16) archsimd.Int64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Int64x4{}
	}
	return x.ConcatPermuteScalarsGrouped(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateconcatSelectedConstantGroupedInt64x8(x, y archsimd.Int64x8, c uint16) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return x.ConcatPermuteScalarsGrouped(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateconcatSelectedConstantGroupedUint32x8(x, y archsimd.Uint32x8, c uint16) archsimd.Uint32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint32x8{}
	}
	return x.ConcatPermuteScalarsGrouped(uint8(c)&7, uint8(c>>3)&7, uint8(c>>6)&7, uint8(c>>9)&7, y)
}

//go:noinline
func immediateconcatSelectedConstantGroupedUint32x16(x, y archsimd.Uint32x16, c uint16) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return x.ConcatPermuteScalarsGrouped(uint8(c)&7, uint8(c>>3)&7, uint8(c>>6)&7, uint8(c>>9)&7, y)
}

//go:noinline
func immediateconcatSelectedConstantGroupedUint64x4(x, y archsimd.Uint64x4, c uint16) archsimd.Uint64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint64x4{}
	}
	return x.ConcatPermuteScalarsGrouped(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateconcatSelectedConstantGroupedUint64x8(x, y archsimd.Uint64x8, c uint16) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return x.ConcatPermuteScalarsGrouped(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateconcatSelectedConstantInt32x4(x, y archsimd.Int32x4, c uint16) archsimd.Int32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x4{}
	}
	return x.ConcatPermuteScalars(uint8(c)&7, uint8(c>>3)&7, uint8(c>>6)&7, uint8(c>>9)&7, y)
}

//go:noinline
func immediateconcatSelectedConstantInt64x2(x, y archsimd.Int64x2, c uint16) archsimd.Int64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Int64x2{}
	}
	return x.ConcatPermuteScalars(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediateconcatSelectedConstantUint32x4(x, y archsimd.Uint32x4, c uint16) archsimd.Uint32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint32x4{}
	}
	return x.ConcatPermuteScalars(uint8(c)&7, uint8(c>>3)&7, uint8(c>>6)&7, uint8(c>>9)&7, y)
}

//go:noinline
func immediateconcatSelectedConstantUint64x2(x, y archsimd.Uint64x2, c uint16) archsimd.Uint64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint64x2{}
	}
	return x.ConcatPermuteScalars(uint8(c)&3, uint8(c>>2)&3, y)
}

//go:noinline
func immediatepermuteScalarsGroupedInt32x8(x, y archsimd.Int32x8, c uint16) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return x.PermuteScalarsGrouped(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsGroupedInt32x16(x, y archsimd.Int32x16, c uint16) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return x.PermuteScalarsGrouped(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsGroupedUint32x8(x, y archsimd.Uint32x8, c uint16) archsimd.Uint32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x8{}
	}
	return x.PermuteScalarsGrouped(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsGroupedUint32x16(x, y archsimd.Uint32x16, c uint16) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return x.PermuteScalarsGrouped(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsHiGroupedInt16x16(x, y archsimd.Int16x16, c uint16) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return x.PermuteScalarsHiGrouped(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsHiGroupedInt16x32(x, y archsimd.Int16x32, c uint16) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return x.PermuteScalarsHiGrouped(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsHiGroupedUint16x16(x, y archsimd.Uint16x16, c uint16) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return x.PermuteScalarsHiGrouped(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsHiGroupedUint16x32(x, y archsimd.Uint16x32, c uint16) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return x.PermuteScalarsHiGrouped(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsHiInt16x8(x, y archsimd.Int16x8, c uint16) archsimd.Int16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Int16x8{}
	}
	return x.PermuteScalarsHi(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsHiUint16x8(x, y archsimd.Uint16x8, c uint16) archsimd.Uint16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint16x8{}
	}
	return x.PermuteScalarsHi(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsInt32x4(x, y archsimd.Int32x4, c uint16) archsimd.Int32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x4{}
	}
	return x.PermuteScalars(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsLoGroupedInt16x16(x, y archsimd.Int16x16, c uint16) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return x.PermuteScalarsLoGrouped(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsLoGroupedInt16x32(x, y archsimd.Int16x32, c uint16) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return x.PermuteScalarsLoGrouped(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsLoGroupedUint16x16(x, y archsimd.Uint16x16, c uint16) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return x.PermuteScalarsLoGrouped(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsLoGroupedUint16x32(x, y archsimd.Uint16x32, c uint16) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return x.PermuteScalarsLoGrouped(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsLoInt16x8(x, y archsimd.Int16x8, c uint16) archsimd.Int16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Int16x8{}
	}
	return x.PermuteScalarsLo(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsLoUint16x8(x, y archsimd.Uint16x8, c uint16) archsimd.Uint16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint16x8{}
	}
	return x.PermuteScalarsLo(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}

//go:noinline
func immediatepermuteScalarsUint32x4(x, y archsimd.Uint32x4, c uint16) archsimd.Uint32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint32x4{}
	}
	return x.PermuteScalars(uint8(c)&3, uint8(c>>2)&3, uint8(c>>4)&3, uint8(c>>6)&3)
}
func checkArch() {
	checkImmediate("ConcatPermute128ScalarsFloat32x8", "ConcatPermute128Scalars", 32, 8, 16, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []float32, c uint16) {
		immediateConcatPermute128ScalarsFloat32x8(archsimd.LoadFloat32x8(x), archsimd.LoadFloat32x8(y), c).Store(got)
	})
	checkImmediate("ConcatPermute128ScalarsFloat64x4", "ConcatPermute128Scalars", 64, 4, 16, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []float64, c uint16) {
		immediateConcatPermute128ScalarsFloat64x4(archsimd.LoadFloat64x4(x), archsimd.LoadFloat64x4(y), c).Store(got)
	})
	checkImmediate("ConcatPermute128ScalarsInt8x32", "ConcatPermute128Scalars", 8, 32, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int8, c uint16) {
		immediateConcatPermute128ScalarsInt8x32(archsimd.LoadInt8x32(x), archsimd.LoadInt8x32(y), c).Store(got)
	})
	checkImmediate("ConcatPermute128ScalarsInt16x16", "ConcatPermute128Scalars", 16, 16, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int16, c uint16) {
		immediateConcatPermute128ScalarsInt16x16(archsimd.LoadInt16x16(x), archsimd.LoadInt16x16(y), c).Store(got)
	})
	checkImmediate("ConcatPermute128ScalarsInt32x8", "ConcatPermute128Scalars", 32, 8, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int32, c uint16) {
		immediateConcatPermute128ScalarsInt32x8(archsimd.LoadInt32x8(x), archsimd.LoadInt32x8(y), c).Store(got)
	})
	checkImmediate("ConcatPermute128ScalarsInt64x4", "ConcatPermute128Scalars", 64, 4, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int64, c uint16) {
		immediateConcatPermute128ScalarsInt64x4(archsimd.LoadInt64x4(x), archsimd.LoadInt64x4(y), c).Store(got)
	})
	checkImmediate("ConcatPermute128ScalarsUint8x32", "ConcatPermute128Scalars", 8, 32, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint8, c uint16) {
		immediateConcatPermute128ScalarsUint8x32(archsimd.LoadUint8x32(x), archsimd.LoadUint8x32(y), c).Store(got)
	})
	checkImmediate("ConcatPermute128ScalarsUint16x16", "ConcatPermute128Scalars", 16, 16, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint16, c uint16) {
		immediateConcatPermute128ScalarsUint16x16(archsimd.LoadUint16x16(x), archsimd.LoadUint16x16(y), c).Store(got)
	})
	checkImmediate("ConcatPermute128ScalarsUint32x8", "ConcatPermute128Scalars", 32, 8, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint32, c uint16) {
		immediateConcatPermute128ScalarsUint32x8(archsimd.LoadUint32x8(x), archsimd.LoadUint32x8(y), c).Store(got)
	})
	checkImmediate("ConcatPermute128ScalarsUint64x4", "ConcatPermute128Scalars", 64, 4, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint64, c uint16) {
		immediateConcatPermute128ScalarsUint64x4(archsimd.LoadUint64x4(x), archsimd.LoadUint64x4(y), c).Store(got)
	})
	checkImmediate("ConcatShiftBytesRightGroupedUint8x32", "ConcatShiftBytesRightGrouped", 8, 32, 256, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint8, c uint16) {
		immediateConcatShiftBytesRightGroupedUint8x32(archsimd.LoadUint8x32(x), archsimd.LoadUint8x32(y), c).Store(got)
	})
	checkImmediate("ConcatShiftBytesRightGroupedUint8x64", "ConcatShiftBytesRightGrouped", 8, 64, 256, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint8, c uint16) {
		immediateConcatShiftBytesRightGroupedUint8x64(archsimd.LoadUint8x64(x), archsimd.LoadUint8x64(y), c).Store(got)
	})
	checkImmediate("ConcatShiftBytesRightUint8x16", "ConcatShiftBytesRight", 8, 16, 256, true, archsimd.X86.AVX(), func(x, y, got []uint8, c uint16) {
		immediateConcatShiftBytesRightUint8x16(archsimd.LoadUint8x16(x), archsimd.LoadUint8x16(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantFloat32x4", "concatSelectedConstant", 32, 4, 4096, true, archsimd.X86.AVX(), func(x, y, got []float32, c uint16) {
		immediateconcatSelectedConstantFloat32x4(archsimd.LoadFloat32x4(x), archsimd.LoadFloat32x4(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantFloat64x2", "concatSelectedConstant", 64, 2, 16, true, archsimd.X86.AVX(), func(x, y, got []float64, c uint16) {
		immediateconcatSelectedConstantFloat64x2(archsimd.LoadFloat64x2(x), archsimd.LoadFloat64x2(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantGroupedFloat32x8", "concatSelectedConstantGrouped", 32, 8, 4096, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []float32, c uint16) {
		immediateconcatSelectedConstantGroupedFloat32x8(archsimd.LoadFloat32x8(x), archsimd.LoadFloat32x8(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantGroupedFloat32x16", "concatSelectedConstantGrouped", 32, 16, 4096, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []float32, c uint16) {
		immediateconcatSelectedConstantGroupedFloat32x16(archsimd.LoadFloat32x16(x), archsimd.LoadFloat32x16(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantGroupedFloat64x4", "concatSelectedConstantGrouped", 64, 4, 16, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []float64, c uint16) {
		immediateconcatSelectedConstantGroupedFloat64x4(archsimd.LoadFloat64x4(x), archsimd.LoadFloat64x4(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantGroupedFloat64x8", "concatSelectedConstantGrouped", 64, 8, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []float64, c uint16) {
		immediateconcatSelectedConstantGroupedFloat64x8(archsimd.LoadFloat64x8(x), archsimd.LoadFloat64x8(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantGroupedInt32x8", "concatSelectedConstantGrouped", 32, 8, 4096, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []int32, c uint16) {
		immediateconcatSelectedConstantGroupedInt32x8(archsimd.LoadInt32x8(x), archsimd.LoadInt32x8(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantGroupedInt32x16", "concatSelectedConstantGrouped", 32, 16, 4096, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int32, c uint16) {
		immediateconcatSelectedConstantGroupedInt32x16(archsimd.LoadInt32x16(x), archsimd.LoadInt32x16(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantGroupedInt64x4", "concatSelectedConstantGrouped", 64, 4, 16, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []int64, c uint16) {
		immediateconcatSelectedConstantGroupedInt64x4(archsimd.LoadInt64x4(x), archsimd.LoadInt64x4(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantGroupedInt64x8", "concatSelectedConstantGrouped", 64, 8, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int64, c uint16) {
		immediateconcatSelectedConstantGroupedInt64x8(archsimd.LoadInt64x8(x), archsimd.LoadInt64x8(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantGroupedUint32x8", "concatSelectedConstantGrouped", 32, 8, 4096, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []uint32, c uint16) {
		immediateconcatSelectedConstantGroupedUint32x8(archsimd.LoadUint32x8(x), archsimd.LoadUint32x8(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantGroupedUint32x16", "concatSelectedConstantGrouped", 32, 16, 4096, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint32, c uint16) {
		immediateconcatSelectedConstantGroupedUint32x16(archsimd.LoadUint32x16(x), archsimd.LoadUint32x16(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantGroupedUint64x4", "concatSelectedConstantGrouped", 64, 4, 16, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []uint64, c uint16) {
		immediateconcatSelectedConstantGroupedUint64x4(archsimd.LoadUint64x4(x), archsimd.LoadUint64x4(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantGroupedUint64x8", "concatSelectedConstantGrouped", 64, 8, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint64, c uint16) {
		immediateconcatSelectedConstantGroupedUint64x8(archsimd.LoadUint64x8(x), archsimd.LoadUint64x8(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantInt32x4", "concatSelectedConstant", 32, 4, 4096, true, archsimd.X86.AVX(), func(x, y, got []int32, c uint16) {
		immediateconcatSelectedConstantInt32x4(archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantInt64x2", "concatSelectedConstant", 64, 2, 16, true, archsimd.X86.AVX(), func(x, y, got []int64, c uint16) {
		immediateconcatSelectedConstantInt64x2(archsimd.LoadInt64x2(x), archsimd.LoadInt64x2(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantUint32x4", "concatSelectedConstant", 32, 4, 4096, true, archsimd.X86.AVX(), func(x, y, got []uint32, c uint16) {
		immediateconcatSelectedConstantUint32x4(archsimd.LoadUint32x4(x), archsimd.LoadUint32x4(y), c).Store(got)
	})
	checkImmediate("concatSelectedConstantUint64x2", "concatSelectedConstant", 64, 2, 16, true, archsimd.X86.AVX(), func(x, y, got []uint64, c uint16) {
		immediateconcatSelectedConstantUint64x2(archsimd.LoadUint64x2(x), archsimd.LoadUint64x2(y), c).Store(got)
	})
	checkImmediate("permuteScalarsGroupedInt32x8", "permuteScalarsGrouped", 32, 8, 256, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int32, c uint16) {
		immediatepermuteScalarsGroupedInt32x8(archsimd.LoadInt32x8(x), archsimd.LoadInt32x8(y), c).Store(got)
	})
	checkImmediate("permuteScalarsGroupedInt32x16", "permuteScalarsGrouped", 32, 16, 256, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int32, c uint16) {
		immediatepermuteScalarsGroupedInt32x16(archsimd.LoadInt32x16(x), archsimd.LoadInt32x16(y), c).Store(got)
	})
	checkImmediate("permuteScalarsGroupedUint32x8", "permuteScalarsGrouped", 32, 8, 256, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint32, c uint16) {
		immediatepermuteScalarsGroupedUint32x8(archsimd.LoadUint32x8(x), archsimd.LoadUint32x8(y), c).Store(got)
	})
	checkImmediate("permuteScalarsGroupedUint32x16", "permuteScalarsGrouped", 32, 16, 256, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint32, c uint16) {
		immediatepermuteScalarsGroupedUint32x16(archsimd.LoadUint32x16(x), archsimd.LoadUint32x16(y), c).Store(got)
	})
	checkImmediate("permuteScalarsHiGroupedInt16x16", "permuteScalarsHiGrouped", 16, 16, 256, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int16, c uint16) {
		immediatepermuteScalarsHiGroupedInt16x16(archsimd.LoadInt16x16(x), archsimd.LoadInt16x16(y), c).Store(got)
	})
	checkImmediate("permuteScalarsHiGroupedInt16x32", "permuteScalarsHiGrouped", 16, 32, 256, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int16, c uint16) {
		immediatepermuteScalarsHiGroupedInt16x32(archsimd.LoadInt16x32(x), archsimd.LoadInt16x32(y), c).Store(got)
	})
	checkImmediate("permuteScalarsHiGroupedUint16x16", "permuteScalarsHiGrouped", 16, 16, 256, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint16, c uint16) {
		immediatepermuteScalarsHiGroupedUint16x16(archsimd.LoadUint16x16(x), archsimd.LoadUint16x16(y), c).Store(got)
	})
	checkImmediate("permuteScalarsHiGroupedUint16x32", "permuteScalarsHiGrouped", 16, 32, 256, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint16, c uint16) {
		immediatepermuteScalarsHiGroupedUint16x32(archsimd.LoadUint16x32(x), archsimd.LoadUint16x32(y), c).Store(got)
	})
	checkImmediate("permuteScalarsHiInt16x8", "permuteScalarsHi", 16, 8, 256, true, archsimd.X86.AVX(), func(x, y, got []int16, c uint16) {
		immediatepermuteScalarsHiInt16x8(archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y), c).Store(got)
	})
	checkImmediate("permuteScalarsHiUint16x8", "permuteScalarsHi", 16, 8, 256, true, archsimd.X86.AVX(), func(x, y, got []uint16, c uint16) {
		immediatepermuteScalarsHiUint16x8(archsimd.LoadUint16x8(x), archsimd.LoadUint16x8(y), c).Store(got)
	})
	checkImmediate("permuteScalarsInt32x4", "permuteScalars", 32, 4, 256, true, archsimd.X86.AVX(), func(x, y, got []int32, c uint16) {
		immediatepermuteScalarsInt32x4(archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y), c).Store(got)
	})
	checkImmediate("permuteScalarsLoGroupedInt16x16", "permuteScalarsLoGrouped", 16, 16, 256, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int16, c uint16) {
		immediatepermuteScalarsLoGroupedInt16x16(archsimd.LoadInt16x16(x), archsimd.LoadInt16x16(y), c).Store(got)
	})
	checkImmediate("permuteScalarsLoGroupedInt16x32", "permuteScalarsLoGrouped", 16, 32, 256, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int16, c uint16) {
		immediatepermuteScalarsLoGroupedInt16x32(archsimd.LoadInt16x32(x), archsimd.LoadInt16x32(y), c).Store(got)
	})
	checkImmediate("permuteScalarsLoGroupedUint16x16", "permuteScalarsLoGrouped", 16, 16, 256, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint16, c uint16) {
		immediatepermuteScalarsLoGroupedUint16x16(archsimd.LoadUint16x16(x), archsimd.LoadUint16x16(y), c).Store(got)
	})
	checkImmediate("permuteScalarsLoGroupedUint16x32", "permuteScalarsLoGrouped", 16, 32, 256, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint16, c uint16) {
		immediatepermuteScalarsLoGroupedUint16x32(archsimd.LoadUint16x32(x), archsimd.LoadUint16x32(y), c).Store(got)
	})
	checkImmediate("permuteScalarsLoInt16x8", "permuteScalarsLo", 16, 8, 256, true, archsimd.X86.AVX(), func(x, y, got []int16, c uint16) {
		immediatepermuteScalarsLoInt16x8(archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y), c).Store(got)
	})
	checkImmediate("permuteScalarsLoUint16x8", "permuteScalarsLo", 16, 8, 256, true, archsimd.X86.AVX(), func(x, y, got []uint16, c uint16) {
		immediatepermuteScalarsLoUint16x8(archsimd.LoadUint16x8(x), archsimd.LoadUint16x8(y), c).Store(got)
	})
	checkImmediate("permuteScalarsUint32x4", "permuteScalars", 32, 4, 256, true, archsimd.X86.AVX(), func(x, y, got []uint32, c uint16) {
		immediatepermuteScalarsUint32x4(archsimd.LoadUint32x4(x), archsimd.LoadUint32x4(y), c).Store(got)
	})
}
