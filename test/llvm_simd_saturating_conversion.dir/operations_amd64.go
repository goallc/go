// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build amd64

package main

import (
	"simd/archsimd"
)

//go:noinline
func SaturateToInt8Int16x16(x archsimd.Int16x16) archsimd.Int8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x16{}
	}
	return x.SaturateToInt8()
}

//go:noinline
func SaturateToInt8Int16x32(x archsimd.Int16x32) archsimd.Int8x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x32{}
	}
	return x.SaturateToInt8()
}

//go:noinline
func SaturateToInt8Int32x4(x archsimd.Int32x4) archsimd.Int8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x16{}
	}
	return x.SaturateToInt8()
}

//go:noinline
func SaturateToInt8Int32x8(x archsimd.Int32x8) archsimd.Int8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x16{}
	}
	return x.SaturateToInt8()
}

//go:noinline
func SaturateToInt8Int32x16(x archsimd.Int32x16) archsimd.Int8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x16{}
	}
	return x.SaturateToInt8()
}

//go:noinline
func SaturateToInt8Int64x2(x archsimd.Int64x2) archsimd.Int8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x16{}
	}
	return x.SaturateToInt8()
}

//go:noinline
func SaturateToInt8Int64x4(x archsimd.Int64x4) archsimd.Int8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x16{}
	}
	return x.SaturateToInt8()
}

//go:noinline
func SaturateToInt8Int64x8(x archsimd.Int64x8) archsimd.Int8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x16{}
	}
	return x.SaturateToInt8()
}

//go:noinline
func SaturateToInt16Int32x8(x archsimd.Int32x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x8{}
	}
	return x.SaturateToInt16()
}

//go:noinline
func SaturateToInt16Int32x16(x archsimd.Int32x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x16{}
	}
	return x.SaturateToInt16()
}

//go:noinline
func SaturateToInt16Int64x2(x archsimd.Int64x2) archsimd.Int16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x8{}
	}
	return x.SaturateToInt16()
}

//go:noinline
func SaturateToInt16Int64x4(x archsimd.Int64x4) archsimd.Int16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x8{}
	}
	return x.SaturateToInt16()
}

//go:noinline
func SaturateToInt16Int64x8(x archsimd.Int64x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x8{}
	}
	return x.SaturateToInt16()
}

//go:noinline
func SaturateToInt16ConcatInt32x4(x archsimd.Int32x4, y archsimd.Int32x4) archsimd.Int16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Int16x8{}
	}
	return x.SaturateToInt16Concat(y)
}

//go:noinline
func SaturateToInt16ConcatGroupedInt32x8(x archsimd.Int32x8, y archsimd.Int32x8) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return x.SaturateToInt16ConcatGrouped(y)
}

//go:noinline
func SaturateToInt16ConcatGroupedInt32x16(x archsimd.Int32x16, y archsimd.Int32x16) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return x.SaturateToInt16ConcatGrouped(y)
}

//go:noinline
func SaturateToInt32Int64x4(x archsimd.Int64x4) archsimd.Int32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x4{}
	}
	return x.SaturateToInt32()
}

//go:noinline
func SaturateToInt32Int64x8(x archsimd.Int64x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x8{}
	}
	return x.SaturateToInt32()
}

//go:noinline
func SaturateToUint8Uint16x16(x archsimd.Uint16x16) archsimd.Uint8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x16{}
	}
	return x.SaturateToUint8()
}

//go:noinline
func SaturateToUint8Uint16x32(x archsimd.Uint16x32) archsimd.Uint8x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x32{}
	}
	return x.SaturateToUint8()
}

//go:noinline
func SaturateToUint8Uint32x4(x archsimd.Uint32x4) archsimd.Uint8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x16{}
	}
	return x.SaturateToUint8()
}

//go:noinline
func SaturateToUint8Uint32x8(x archsimd.Uint32x8) archsimd.Uint8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x16{}
	}
	return x.SaturateToUint8()
}

//go:noinline
func SaturateToUint8Uint32x16(x archsimd.Uint32x16) archsimd.Uint8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x16{}
	}
	return x.SaturateToUint8()
}

//go:noinline
func SaturateToUint8Uint64x2(x archsimd.Uint64x2) archsimd.Uint8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x16{}
	}
	return x.SaturateToUint8()
}

//go:noinline
func SaturateToUint8Uint64x4(x archsimd.Uint64x4) archsimd.Uint8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x16{}
	}
	return x.SaturateToUint8()
}

//go:noinline
func SaturateToUint8Uint64x8(x archsimd.Uint64x8) archsimd.Uint8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x16{}
	}
	return x.SaturateToUint8()
}

//go:noinline
func SaturateToUint16Uint32x8(x archsimd.Uint32x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x8{}
	}
	return x.SaturateToUint16()
}

//go:noinline
func SaturateToUint16Uint32x16(x archsimd.Uint32x16) archsimd.Uint16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x16{}
	}
	return x.SaturateToUint16()
}

//go:noinline
func SaturateToUint16Uint64x2(x archsimd.Uint64x2) archsimd.Uint16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x8{}
	}
	return x.SaturateToUint16()
}

//go:noinline
func SaturateToUint16Uint64x4(x archsimd.Uint64x4) archsimd.Uint16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x8{}
	}
	return x.SaturateToUint16()
}

//go:noinline
func SaturateToUint16Uint64x8(x archsimd.Uint64x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x8{}
	}
	return x.SaturateToUint16()
}

//go:noinline
func SaturateToUint16ConcatInt32x4(x archsimd.Int32x4, y archsimd.Int32x4) archsimd.Uint16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint16x8{}
	}
	return x.SaturateToUint16Concat(y)
}

//go:noinline
func SaturateToUint16ConcatGroupedInt32x8(x archsimd.Int32x8, y archsimd.Int32x8) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return x.SaturateToUint16ConcatGrouped(y)
}

//go:noinline
func SaturateToUint16ConcatGroupedInt32x16(x archsimd.Int32x16, y archsimd.Int32x16) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return x.SaturateToUint16ConcatGrouped(y)
}

//go:noinline
func SaturateToUint32Uint64x4(x archsimd.Uint64x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x4{}
	}
	return x.SaturateToUint32()
}

//go:noinline
func SaturateToUint32Uint64x8(x archsimd.Uint64x8) archsimd.Uint32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x8{}
	}
	return x.SaturateToUint32()
}

func checkArch() {
	check("SaturateToInt8Int16x16", 16, 16, 8, 16, true, true, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []int16, got []int8) {
		result := SaturateToInt8Int16x16(archsimd.LoadInt16x16(x))
		result.Store(got)
	})
	check("SaturateToInt8Int16x32", 16, 32, 8, 32, true, true, false, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []int16, got []int8) {
		result := SaturateToInt8Int16x32(archsimd.LoadInt16x32(x))
		result.Store(got)
	})
	check("SaturateToInt8Int32x4", 32, 4, 8, 16, true, true, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []int32, got []int8) {
		result := SaturateToInt8Int32x4(archsimd.LoadInt32x4(x))
		result.Store(got)
	})
	check("SaturateToInt8Int32x8", 32, 8, 8, 16, true, true, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []int32, got []int8) {
		result := SaturateToInt8Int32x8(archsimd.LoadInt32x8(x))
		result.Store(got)
	})
	check("SaturateToInt8Int32x16", 32, 16, 8, 16, true, true, false, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []int32, got []int8) {
		result := SaturateToInt8Int32x16(archsimd.LoadInt32x16(x))
		result.Store(got)
	})
	check("SaturateToInt8Int64x2", 64, 2, 8, 16, true, true, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []int64, got []int8) {
		result := SaturateToInt8Int64x2(archsimd.LoadInt64x2(x))
		result.Store(got)
	})
	check("SaturateToInt8Int64x4", 64, 4, 8, 16, true, true, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []int64, got []int8) {
		result := SaturateToInt8Int64x4(archsimd.LoadInt64x4(x))
		result.Store(got)
	})
	check("SaturateToInt8Int64x8", 64, 8, 8, 16, true, true, false, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []int64, got []int8) {
		result := SaturateToInt8Int64x8(archsimd.LoadInt64x8(x))
		result.Store(got)
	})
	check("SaturateToInt16Int32x8", 32, 8, 16, 8, true, true, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []int32, got []int16) {
		result := SaturateToInt16Int32x8(archsimd.LoadInt32x8(x))
		result.Store(got)
	})
	check("SaturateToInt16Int32x16", 32, 16, 16, 16, true, true, false, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []int32, got []int16) {
		result := SaturateToInt16Int32x16(archsimd.LoadInt32x16(x))
		result.Store(got)
	})
	check("SaturateToInt16Int64x2", 64, 2, 16, 8, true, true, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []int64, got []int16) {
		result := SaturateToInt16Int64x2(archsimd.LoadInt64x2(x))
		result.Store(got)
	})
	check("SaturateToInt16Int64x4", 64, 4, 16, 8, true, true, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []int64, got []int16) {
		result := SaturateToInt16Int64x4(archsimd.LoadInt64x4(x))
		result.Store(got)
	})
	check("SaturateToInt16Int64x8", 64, 8, 16, 8, true, true, false, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []int64, got []int16) {
		result := SaturateToInt16Int64x8(archsimd.LoadInt64x8(x))
		result.Store(got)
	})
	check("SaturateToInt16ConcatInt32x4", 32, 4, 16, 8, true, true, true, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y []int32, got []int16) {
		result := SaturateToInt16ConcatInt32x4(archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y))
		result.Store(got)
	})
	check("SaturateToInt16ConcatGroupedInt32x8", 32, 8, 16, 16, true, true, true, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y []int32, got []int16) {
		result := SaturateToInt16ConcatGroupedInt32x8(archsimd.LoadInt32x8(x), archsimd.LoadInt32x8(y))
		result.Store(got)
	})
	check("SaturateToInt16ConcatGroupedInt32x16", 32, 16, 16, 32, true, true, true, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []int32, got []int16) {
		result := SaturateToInt16ConcatGroupedInt32x16(archsimd.LoadInt32x16(x), archsimd.LoadInt32x16(y))
		result.Store(got)
	})
	check("SaturateToInt32Int64x4", 64, 4, 32, 4, true, true, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []int64, got []int32) {
		result := SaturateToInt32Int64x4(archsimd.LoadInt64x4(x))
		result.Store(got)
	})
	check("SaturateToInt32Int64x8", 64, 8, 32, 8, true, true, false, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []int64, got []int32) {
		result := SaturateToInt32Int64x8(archsimd.LoadInt64x8(x))
		result.Store(got)
	})
	check("SaturateToUint8Uint16x16", 16, 16, 8, 16, false, false, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []uint16, got []uint8) {
		result := SaturateToUint8Uint16x16(archsimd.LoadUint16x16(x))
		result.Store(got)
	})
	check("SaturateToUint8Uint16x32", 16, 32, 8, 32, false, false, false, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []uint16, got []uint8) {
		result := SaturateToUint8Uint16x32(archsimd.LoadUint16x32(x))
		result.Store(got)
	})
	check("SaturateToUint8Uint32x4", 32, 4, 8, 16, false, false, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []uint32, got []uint8) {
		result := SaturateToUint8Uint32x4(archsimd.LoadUint32x4(x))
		result.Store(got)
	})
	check("SaturateToUint8Uint32x8", 32, 8, 8, 16, false, false, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []uint32, got []uint8) {
		result := SaturateToUint8Uint32x8(archsimd.LoadUint32x8(x))
		result.Store(got)
	})
	check("SaturateToUint8Uint32x16", 32, 16, 8, 16, false, false, false, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []uint32, got []uint8) {
		result := SaturateToUint8Uint32x16(archsimd.LoadUint32x16(x))
		result.Store(got)
	})
	check("SaturateToUint8Uint64x2", 64, 2, 8, 16, false, false, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []uint64, got []uint8) {
		result := SaturateToUint8Uint64x2(archsimd.LoadUint64x2(x))
		result.Store(got)
	})
	check("SaturateToUint8Uint64x4", 64, 4, 8, 16, false, false, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []uint64, got []uint8) {
		result := SaturateToUint8Uint64x4(archsimd.LoadUint64x4(x))
		result.Store(got)
	})
	check("SaturateToUint8Uint64x8", 64, 8, 8, 16, false, false, false, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []uint64, got []uint8) {
		result := SaturateToUint8Uint64x8(archsimd.LoadUint64x8(x))
		result.Store(got)
	})
	check("SaturateToUint16Uint32x8", 32, 8, 16, 8, false, false, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []uint32, got []uint16) {
		result := SaturateToUint16Uint32x8(archsimd.LoadUint32x8(x))
		result.Store(got)
	})
	check("SaturateToUint16Uint32x16", 32, 16, 16, 16, false, false, false, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []uint32, got []uint16) {
		result := SaturateToUint16Uint32x16(archsimd.LoadUint32x16(x))
		result.Store(got)
	})
	check("SaturateToUint16Uint64x2", 64, 2, 16, 8, false, false, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []uint64, got []uint16) {
		result := SaturateToUint16Uint64x2(archsimd.LoadUint64x2(x))
		result.Store(got)
	})
	check("SaturateToUint16Uint64x4", 64, 4, 16, 8, false, false, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []uint64, got []uint16) {
		result := SaturateToUint16Uint64x4(archsimd.LoadUint64x4(x))
		result.Store(got)
	})
	check("SaturateToUint16Uint64x8", 64, 8, 16, 8, false, false, false, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []uint64, got []uint16) {
		result := SaturateToUint16Uint64x8(archsimd.LoadUint64x8(x))
		result.Store(got)
	})
	check("SaturateToUint16ConcatInt32x4", 32, 4, 16, 8, true, false, true, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y []int32, got []uint16) {
		result := SaturateToUint16ConcatInt32x4(archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y))
		result.Store(got)
	})
	check("SaturateToUint16ConcatGroupedInt32x8", 32, 8, 16, 16, true, false, true, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y []int32, got []uint16) {
		result := SaturateToUint16ConcatGroupedInt32x8(archsimd.LoadInt32x8(x), archsimd.LoadInt32x8(y))
		result.Store(got)
	})
	check("SaturateToUint16ConcatGroupedInt32x16", 32, 16, 16, 32, true, false, true, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []int32, got []uint16) {
		result := SaturateToUint16ConcatGroupedInt32x16(archsimd.LoadInt32x16(x), archsimd.LoadInt32x16(y))
		result.Store(got)
	})
	check("SaturateToUint32Uint64x4", 64, 4, 32, 4, false, false, false, archsimd.X86.AVX(), archsimd.X86.AVX512(), func(x, y []uint64, got []uint32) {
		result := SaturateToUint32Uint64x4(archsimd.LoadUint64x4(x))
		result.Store(got)
	})
	check("SaturateToUint32Uint64x8", 64, 8, 32, 8, false, false, false, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y []uint64, got []uint32) {
		result := SaturateToUint32Uint64x8(archsimd.LoadUint64x8(x))
		result.Store(got)
	})
}
