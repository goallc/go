// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func shuffleGetHiFloat32x8(x archsimd.Float32x8) archsimd.Float32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x4{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiFloat32x16(x archsimd.Float32x16) archsimd.Float32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x8{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiFloat64x4(x archsimd.Float64x4) archsimd.Float64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x2{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiFloat64x8(x archsimd.Float64x8) archsimd.Float64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x4{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiInt8x32(x archsimd.Int8x32) archsimd.Int8x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int8x16{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiInt8x64(x archsimd.Int8x64) archsimd.Int8x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x32{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiInt16x16(x archsimd.Int16x16) archsimd.Int16x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x8{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiInt16x32(x archsimd.Int16x32) archsimd.Int16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x16{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiInt32x8(x archsimd.Int32x8) archsimd.Int32x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x4{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiInt32x16(x archsimd.Int32x16) archsimd.Int32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x8{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiInt64x4(x archsimd.Int64x4) archsimd.Int64x2 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x2{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiInt64x8(x archsimd.Int64x8) archsimd.Int64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x4{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiUint8x32(x archsimd.Uint8x32) archsimd.Uint8x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x16{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiUint8x64(x archsimd.Uint8x64) archsimd.Uint8x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x32{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiUint16x16(x archsimd.Uint16x16) archsimd.Uint16x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x8{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiUint16x32(x archsimd.Uint16x32) archsimd.Uint16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x16{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiUint32x8(x archsimd.Uint32x8) archsimd.Uint32x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x4{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiUint32x16(x archsimd.Uint32x16) archsimd.Uint32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x8{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiUint64x4(x archsimd.Uint64x4) archsimd.Uint64x2 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x2{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetHiUint64x8(x archsimd.Uint64x8) archsimd.Uint64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x4{}
	}
	return x.GetHi()
}

//go:noinline
func shuffleGetLoFloat32x8(x archsimd.Float32x8) archsimd.Float32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x4{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoFloat32x16(x archsimd.Float32x16) archsimd.Float32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x8{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoFloat64x4(x archsimd.Float64x4) archsimd.Float64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x2{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoFloat64x8(x archsimd.Float64x8) archsimd.Float64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x4{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoInt8x32(x archsimd.Int8x32) archsimd.Int8x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int8x16{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoInt8x64(x archsimd.Int8x64) archsimd.Int8x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x32{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoInt16x16(x archsimd.Int16x16) archsimd.Int16x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x8{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoInt16x32(x archsimd.Int16x32) archsimd.Int16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x16{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoInt32x8(x archsimd.Int32x8) archsimd.Int32x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x4{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoInt32x16(x archsimd.Int32x16) archsimd.Int32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x8{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoInt64x4(x archsimd.Int64x4) archsimd.Int64x2 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x2{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoInt64x8(x archsimd.Int64x8) archsimd.Int64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x4{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoUint8x32(x archsimd.Uint8x32) archsimd.Uint8x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x16{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoUint8x64(x archsimd.Uint8x64) archsimd.Uint8x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x32{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoUint16x16(x archsimd.Uint16x16) archsimd.Uint16x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x8{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoUint16x32(x archsimd.Uint16x32) archsimd.Uint16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x16{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoUint32x8(x archsimd.Uint32x8) archsimd.Uint32x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x4{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoUint32x16(x archsimd.Uint32x16) archsimd.Uint32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x8{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoUint64x4(x archsimd.Uint64x4) archsimd.Uint64x2 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x2{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleGetLoUint64x8(x archsimd.Uint64x8) archsimd.Uint64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x4{}
	}
	return x.GetLo()
}

//go:noinline
func shuffleInterleaveHiInt16x8(x archsimd.Int16x8, y archsimd.Int16x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Int16x8{}
	}
	return x.InterleaveHi(y)
}

//go:noinline
func shuffleInterleaveHiInt32x4(x archsimd.Int32x4, y archsimd.Int32x4) archsimd.Int32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x4{}
	}
	return x.InterleaveHi(y)
}

//go:noinline
func shuffleInterleaveHiInt64x2(x archsimd.Int64x2, y archsimd.Int64x2) archsimd.Int64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Int64x2{}
	}
	return x.InterleaveHi(y)
}

//go:noinline
func shuffleInterleaveHiUint16x8(x archsimd.Uint16x8, y archsimd.Uint16x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint16x8{}
	}
	return x.InterleaveHi(y)
}

//go:noinline
func shuffleInterleaveHiUint32x4(x archsimd.Uint32x4, y archsimd.Uint32x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint32x4{}
	}
	return x.InterleaveHi(y)
}

//go:noinline
func shuffleInterleaveHiUint64x2(x archsimd.Uint64x2, y archsimd.Uint64x2) archsimd.Uint64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint64x2{}
	}
	return x.InterleaveHi(y)
}

//go:noinline
func shuffleInterleaveHiGroupedInt16x16(x archsimd.Int16x16, y archsimd.Int16x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return x.InterleaveHiGrouped(y)
}

//go:noinline
func shuffleInterleaveHiGroupedInt16x32(x archsimd.Int16x32, y archsimd.Int16x32) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return x.InterleaveHiGrouped(y)
}

//go:noinline
func shuffleInterleaveHiGroupedInt32x8(x archsimd.Int32x8, y archsimd.Int32x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return x.InterleaveHiGrouped(y)
}

//go:noinline
func shuffleInterleaveHiGroupedInt32x16(x archsimd.Int32x16, y archsimd.Int32x16) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return x.InterleaveHiGrouped(y)
}

//go:noinline
func shuffleInterleaveHiGroupedInt64x4(x archsimd.Int64x4, y archsimd.Int64x4) archsimd.Int64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x4{}
	}
	return x.InterleaveHiGrouped(y)
}

//go:noinline
func shuffleInterleaveHiGroupedInt64x8(x archsimd.Int64x8, y archsimd.Int64x8) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return x.InterleaveHiGrouped(y)
}

//go:noinline
func shuffleInterleaveHiGroupedUint16x16(x archsimd.Uint16x16, y archsimd.Uint16x16) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return x.InterleaveHiGrouped(y)
}

//go:noinline
func shuffleInterleaveHiGroupedUint16x32(x archsimd.Uint16x32, y archsimd.Uint16x32) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return x.InterleaveHiGrouped(y)
}

//go:noinline
func shuffleInterleaveHiGroupedUint32x8(x archsimd.Uint32x8, y archsimd.Uint32x8) archsimd.Uint32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x8{}
	}
	return x.InterleaveHiGrouped(y)
}

//go:noinline
func shuffleInterleaveHiGroupedUint32x16(x archsimd.Uint32x16, y archsimd.Uint32x16) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return x.InterleaveHiGrouped(y)
}

//go:noinline
func shuffleInterleaveHiGroupedUint64x4(x archsimd.Uint64x4, y archsimd.Uint64x4) archsimd.Uint64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x4{}
	}
	return x.InterleaveHiGrouped(y)
}

//go:noinline
func shuffleInterleaveHiGroupedUint64x8(x archsimd.Uint64x8, y archsimd.Uint64x8) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return x.InterleaveHiGrouped(y)
}

//go:noinline
func shuffleInterleaveLoInt16x8(x archsimd.Int16x8, y archsimd.Int16x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Int16x8{}
	}
	return x.InterleaveLo(y)
}

//go:noinline
func shuffleInterleaveLoInt32x4(x archsimd.Int32x4, y archsimd.Int32x4) archsimd.Int32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x4{}
	}
	return x.InterleaveLo(y)
}

//go:noinline
func shuffleInterleaveLoInt64x2(x archsimd.Int64x2, y archsimd.Int64x2) archsimd.Int64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Int64x2{}
	}
	return x.InterleaveLo(y)
}

//go:noinline
func shuffleInterleaveLoUint16x8(x archsimd.Uint16x8, y archsimd.Uint16x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint16x8{}
	}
	return x.InterleaveLo(y)
}

//go:noinline
func shuffleInterleaveLoUint32x4(x archsimd.Uint32x4, y archsimd.Uint32x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint32x4{}
	}
	return x.InterleaveLo(y)
}

//go:noinline
func shuffleInterleaveLoUint64x2(x archsimd.Uint64x2, y archsimd.Uint64x2) archsimd.Uint64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint64x2{}
	}
	return x.InterleaveLo(y)
}

//go:noinline
func shuffleInterleaveLoGroupedInt16x16(x archsimd.Int16x16, y archsimd.Int16x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return x.InterleaveLoGrouped(y)
}

//go:noinline
func shuffleInterleaveLoGroupedInt16x32(x archsimd.Int16x32, y archsimd.Int16x32) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return x.InterleaveLoGrouped(y)
}

//go:noinline
func shuffleInterleaveLoGroupedInt32x8(x archsimd.Int32x8, y archsimd.Int32x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return x.InterleaveLoGrouped(y)
}

//go:noinline
func shuffleInterleaveLoGroupedInt32x16(x archsimd.Int32x16, y archsimd.Int32x16) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return x.InterleaveLoGrouped(y)
}

//go:noinline
func shuffleInterleaveLoGroupedInt64x4(x archsimd.Int64x4, y archsimd.Int64x4) archsimd.Int64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x4{}
	}
	return x.InterleaveLoGrouped(y)
}

//go:noinline
func shuffleInterleaveLoGroupedInt64x8(x archsimd.Int64x8, y archsimd.Int64x8) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return x.InterleaveLoGrouped(y)
}

//go:noinline
func shuffleInterleaveLoGroupedUint16x16(x archsimd.Uint16x16, y archsimd.Uint16x16) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return x.InterleaveLoGrouped(y)
}

//go:noinline
func shuffleInterleaveLoGroupedUint16x32(x archsimd.Uint16x32, y archsimd.Uint16x32) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return x.InterleaveLoGrouped(y)
}

//go:noinline
func shuffleInterleaveLoGroupedUint32x8(x archsimd.Uint32x8, y archsimd.Uint32x8) archsimd.Uint32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x8{}
	}
	return x.InterleaveLoGrouped(y)
}

//go:noinline
func shuffleInterleaveLoGroupedUint32x16(x archsimd.Uint32x16, y archsimd.Uint32x16) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return x.InterleaveLoGrouped(y)
}

//go:noinline
func shuffleInterleaveLoGroupedUint64x4(x archsimd.Uint64x4, y archsimd.Uint64x4) archsimd.Uint64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x4{}
	}
	return x.InterleaveLoGrouped(y)
}

//go:noinline
func shuffleInterleaveLoGroupedUint64x8(x archsimd.Uint64x8, y archsimd.Uint64x8) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return x.InterleaveLoGrouped(y)
}

//go:noinline
func shuffleSetHiFloat32x8(x archsimd.Float32x8, y archsimd.Float32x4) archsimd.Float32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x8{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiFloat32x16(x archsimd.Float32x16, y archsimd.Float32x8) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiFloat64x4(x archsimd.Float64x4, y archsimd.Float64x2) archsimd.Float64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x4{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiFloat64x8(x archsimd.Float64x8, y archsimd.Float64x4) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiInt8x32(x archsimd.Int8x32, y archsimd.Int8x16) archsimd.Int8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int8x32{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiInt8x64(x archsimd.Int8x64, y archsimd.Int8x32) archsimd.Int8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x64{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiInt16x16(x archsimd.Int16x16, y archsimd.Int16x8) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiInt16x32(x archsimd.Int16x32, y archsimd.Int16x16) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiInt32x8(x archsimd.Int32x8, y archsimd.Int32x4) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiInt32x16(x archsimd.Int32x16, y archsimd.Int32x8) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiInt64x4(x archsimd.Int64x4, y archsimd.Int64x2) archsimd.Int64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x4{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiInt64x8(x archsimd.Int64x8, y archsimd.Int64x4) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiUint8x32(x archsimd.Uint8x32, y archsimd.Uint8x16) archsimd.Uint8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x32{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiUint8x64(x archsimd.Uint8x64, y archsimd.Uint8x32) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x64{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiUint16x16(x archsimd.Uint16x16, y archsimd.Uint16x8) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiUint16x32(x archsimd.Uint16x32, y archsimd.Uint16x16) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiUint32x8(x archsimd.Uint32x8, y archsimd.Uint32x4) archsimd.Uint32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x8{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiUint32x16(x archsimd.Uint32x16, y archsimd.Uint32x8) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiUint64x4(x archsimd.Uint64x4, y archsimd.Uint64x2) archsimd.Uint64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x4{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetHiUint64x8(x archsimd.Uint64x8, y archsimd.Uint64x4) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return x.SetHi(y)
}

//go:noinline
func shuffleSetLoFloat32x8(x archsimd.Float32x8, y archsimd.Float32x4) archsimd.Float32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x8{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoFloat32x16(x archsimd.Float32x16, y archsimd.Float32x8) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoFloat64x4(x archsimd.Float64x4, y archsimd.Float64x2) archsimd.Float64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x4{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoFloat64x8(x archsimd.Float64x8, y archsimd.Float64x4) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoInt8x32(x archsimd.Int8x32, y archsimd.Int8x16) archsimd.Int8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int8x32{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoInt8x64(x archsimd.Int8x64, y archsimd.Int8x32) archsimd.Int8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x64{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoInt16x16(x archsimd.Int16x16, y archsimd.Int16x8) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoInt16x32(x archsimd.Int16x32, y archsimd.Int16x16) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoInt32x8(x archsimd.Int32x8, y archsimd.Int32x4) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoInt32x16(x archsimd.Int32x16, y archsimd.Int32x8) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoInt64x4(x archsimd.Int64x4, y archsimd.Int64x2) archsimd.Int64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x4{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoInt64x8(x archsimd.Int64x8, y archsimd.Int64x4) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoUint8x32(x archsimd.Uint8x32, y archsimd.Uint8x16) archsimd.Uint8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x32{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoUint8x64(x archsimd.Uint8x64, y archsimd.Uint8x32) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x64{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoUint16x16(x archsimd.Uint16x16, y archsimd.Uint16x8) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoUint16x32(x archsimd.Uint16x32, y archsimd.Uint16x16) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoUint32x8(x archsimd.Uint32x8, y archsimd.Uint32x4) archsimd.Uint32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x8{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoUint32x16(x archsimd.Uint32x16, y archsimd.Uint32x8) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoUint64x4(x archsimd.Uint64x4, y archsimd.Uint64x2) archsimd.Uint64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x4{}
	}
	return x.SetLo(y)
}

//go:noinline
func shuffleSetLoUint64x8(x archsimd.Uint64x8, y archsimd.Uint64x4) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return x.SetLo(y)
}

//go:noinline
func shufflebroadcast1To2Float64x2(x archsimd.Float64x2) archsimd.Float64x2 {
	if !archsimd.X86.AVX2() {
		return archsimd.Float64x2{}
	}
	return archsimd.BroadcastFloat64x2(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To2Int64x2(x archsimd.Int64x2) archsimd.Int64x2 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x2{}
	}
	return archsimd.BroadcastInt64x2(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To2Uint64x2(x archsimd.Uint64x2) archsimd.Uint64x2 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x2{}
	}
	return archsimd.BroadcastUint64x2(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To4Float32x4(x archsimd.Float32x4) archsimd.Float32x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Float32x4{}
	}
	return archsimd.BroadcastFloat32x4(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To4Float64x2(x archsimd.Float64x2) archsimd.Float64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Float64x4{}
	}
	return archsimd.BroadcastFloat64x4(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To4Int32x4(x archsimd.Int32x4) archsimd.Int32x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x4{}
	}
	return archsimd.BroadcastInt32x4(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To4Int64x2(x archsimd.Int64x2) archsimd.Int64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x4{}
	}
	return archsimd.BroadcastInt64x4(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To4Uint32x4(x archsimd.Uint32x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x4{}
	}
	return archsimd.BroadcastUint32x4(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To4Uint64x2(x archsimd.Uint64x2) archsimd.Uint64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x4{}
	}
	return archsimd.BroadcastUint64x4(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To8Float32x4(x archsimd.Float32x4) archsimd.Float32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Float32x8{}
	}
	return archsimd.BroadcastFloat32x8(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To8Float64x2(x archsimd.Float64x2) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return archsimd.BroadcastFloat64x8(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To8Int16x8(x archsimd.Int16x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x8{}
	}
	return archsimd.BroadcastInt16x8(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To8Int32x4(x archsimd.Int32x4) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return archsimd.BroadcastInt32x8(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To8Int64x2(x archsimd.Int64x2) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return archsimd.BroadcastInt64x8(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To8Uint16x8(x archsimd.Uint16x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x8{}
	}
	return archsimd.BroadcastUint16x8(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To8Uint32x4(x archsimd.Uint32x4) archsimd.Uint32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x8{}
	}
	return archsimd.BroadcastUint32x8(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To8Uint64x2(x archsimd.Uint64x2) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return archsimd.BroadcastUint64x8(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To16Float32x4(x archsimd.Float32x4) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return archsimd.BroadcastFloat32x16(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To16Int8x16(x archsimd.Int8x16) archsimd.Int8x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int8x16{}
	}
	return archsimd.BroadcastInt8x16(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To16Int16x8(x archsimd.Int16x8) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return archsimd.BroadcastInt16x16(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To16Int32x4(x archsimd.Int32x4) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return archsimd.BroadcastInt32x16(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To16Uint8x16(x archsimd.Uint8x16) archsimd.Uint8x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x16{}
	}
	return archsimd.BroadcastUint8x16(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To16Uint16x8(x archsimd.Uint16x8) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return archsimd.BroadcastUint16x16(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To16Uint32x4(x archsimd.Uint32x4) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return archsimd.BroadcastUint32x16(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To32Int8x16(x archsimd.Int8x16) archsimd.Int8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int8x32{}
	}
	return archsimd.BroadcastInt8x32(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To32Int16x8(x archsimd.Int16x8) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return archsimd.BroadcastInt16x32(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To32Uint8x16(x archsimd.Uint8x16) archsimd.Uint8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x32{}
	}
	return archsimd.BroadcastUint8x32(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To32Uint16x8(x archsimd.Uint16x8) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return archsimd.BroadcastUint16x32(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To64Int8x16(x archsimd.Int8x16) archsimd.Int8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x64{}
	}
	return archsimd.BroadcastInt8x64(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To64Uint8x16(x archsimd.Uint8x16) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x64{}
	}
	return archsimd.BroadcastUint8x64(x.GetElem(0))
}

func checkArch() {
	check("GetHiFloat32x8", "get-high", 32, 8, 0, 4, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []float32) {
		shuffleGetHiFloat32x8(archsimd.LoadFloat32x8(x)).Store(got)
	})
	check("GetHiFloat32x16", "get-high", 32, 16, 0, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []float32) {
		shuffleGetHiFloat32x16(archsimd.LoadFloat32x16(x)).Store(got)
	})
	check("GetHiFloat64x4", "get-high", 64, 4, 0, 2, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []float64) {
		shuffleGetHiFloat64x4(archsimd.LoadFloat64x4(x)).Store(got)
	})
	check("GetHiFloat64x8", "get-high", 64, 8, 0, 4, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []float64) {
		shuffleGetHiFloat64x8(archsimd.LoadFloat64x8(x)).Store(got)
	})
	check("GetHiInt8x32", "get-high", 8, 32, 0, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int8) {
		shuffleGetHiInt8x32(archsimd.LoadInt8x32(x)).Store(got)
	})
	check("GetHiInt8x64", "get-high", 8, 64, 0, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int8) {
		shuffleGetHiInt8x64(archsimd.LoadInt8x64(x)).Store(got)
	})
	check("GetHiInt16x16", "get-high", 16, 16, 0, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int16) {
		shuffleGetHiInt16x16(archsimd.LoadInt16x16(x)).Store(got)
	})
	check("GetHiInt16x32", "get-high", 16, 32, 0, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int16) {
		shuffleGetHiInt16x32(archsimd.LoadInt16x32(x)).Store(got)
	})
	check("GetHiInt32x8", "get-high", 32, 8, 0, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int32) {
		shuffleGetHiInt32x8(archsimd.LoadInt32x8(x)).Store(got)
	})
	check("GetHiInt32x16", "get-high", 32, 16, 0, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int32) {
		shuffleGetHiInt32x16(archsimd.LoadInt32x16(x)).Store(got)
	})
	check("GetHiInt64x4", "get-high", 64, 4, 0, 2, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int64) {
		shuffleGetHiInt64x4(archsimd.LoadInt64x4(x)).Store(got)
	})
	check("GetHiInt64x8", "get-high", 64, 8, 0, 4, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int64) {
		shuffleGetHiInt64x8(archsimd.LoadInt64x8(x)).Store(got)
	})
	check("GetHiUint8x32", "get-high", 8, 32, 0, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint8) {
		shuffleGetHiUint8x32(archsimd.LoadUint8x32(x)).Store(got)
	})
	check("GetHiUint8x64", "get-high", 8, 64, 0, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint8) {
		shuffleGetHiUint8x64(archsimd.LoadUint8x64(x)).Store(got)
	})
	check("GetHiUint16x16", "get-high", 16, 16, 0, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint16) {
		shuffleGetHiUint16x16(archsimd.LoadUint16x16(x)).Store(got)
	})
	check("GetHiUint16x32", "get-high", 16, 32, 0, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint16) {
		shuffleGetHiUint16x32(archsimd.LoadUint16x32(x)).Store(got)
	})
	check("GetHiUint32x8", "get-high", 32, 8, 0, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint32) {
		shuffleGetHiUint32x8(archsimd.LoadUint32x8(x)).Store(got)
	})
	check("GetHiUint32x16", "get-high", 32, 16, 0, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint32) {
		shuffleGetHiUint32x16(archsimd.LoadUint32x16(x)).Store(got)
	})
	check("GetHiUint64x4", "get-high", 64, 4, 0, 2, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint64) {
		shuffleGetHiUint64x4(archsimd.LoadUint64x4(x)).Store(got)
	})
	check("GetHiUint64x8", "get-high", 64, 8, 0, 4, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint64) {
		shuffleGetHiUint64x8(archsimd.LoadUint64x8(x)).Store(got)
	})
	check("GetLoFloat32x8", "get-low", 32, 8, 0, 4, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []float32) {
		shuffleGetLoFloat32x8(archsimd.LoadFloat32x8(x)).Store(got)
	})
	check("GetLoFloat32x16", "get-low", 32, 16, 0, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []float32) {
		shuffleGetLoFloat32x16(archsimd.LoadFloat32x16(x)).Store(got)
	})
	check("GetLoFloat64x4", "get-low", 64, 4, 0, 2, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []float64) {
		shuffleGetLoFloat64x4(archsimd.LoadFloat64x4(x)).Store(got)
	})
	check("GetLoFloat64x8", "get-low", 64, 8, 0, 4, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []float64) {
		shuffleGetLoFloat64x8(archsimd.LoadFloat64x8(x)).Store(got)
	})
	check("GetLoInt8x32", "get-low", 8, 32, 0, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int8) {
		shuffleGetLoInt8x32(archsimd.LoadInt8x32(x)).Store(got)
	})
	check("GetLoInt8x64", "get-low", 8, 64, 0, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int8) {
		shuffleGetLoInt8x64(archsimd.LoadInt8x64(x)).Store(got)
	})
	check("GetLoInt16x16", "get-low", 16, 16, 0, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int16) {
		shuffleGetLoInt16x16(archsimd.LoadInt16x16(x)).Store(got)
	})
	check("GetLoInt16x32", "get-low", 16, 32, 0, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int16) {
		shuffleGetLoInt16x32(archsimd.LoadInt16x32(x)).Store(got)
	})
	check("GetLoInt32x8", "get-low", 32, 8, 0, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int32) {
		shuffleGetLoInt32x8(archsimd.LoadInt32x8(x)).Store(got)
	})
	check("GetLoInt32x16", "get-low", 32, 16, 0, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int32) {
		shuffleGetLoInt32x16(archsimd.LoadInt32x16(x)).Store(got)
	})
	check("GetLoInt64x4", "get-low", 64, 4, 0, 2, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int64) {
		shuffleGetLoInt64x4(archsimd.LoadInt64x4(x)).Store(got)
	})
	check("GetLoInt64x8", "get-low", 64, 8, 0, 4, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int64) {
		shuffleGetLoInt64x8(archsimd.LoadInt64x8(x)).Store(got)
	})
	check("GetLoUint8x32", "get-low", 8, 32, 0, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint8) {
		shuffleGetLoUint8x32(archsimd.LoadUint8x32(x)).Store(got)
	})
	check("GetLoUint8x64", "get-low", 8, 64, 0, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint8) {
		shuffleGetLoUint8x64(archsimd.LoadUint8x64(x)).Store(got)
	})
	check("GetLoUint16x16", "get-low", 16, 16, 0, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint16) {
		shuffleGetLoUint16x16(archsimd.LoadUint16x16(x)).Store(got)
	})
	check("GetLoUint16x32", "get-low", 16, 32, 0, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint16) {
		shuffleGetLoUint16x32(archsimd.LoadUint16x32(x)).Store(got)
	})
	check("GetLoUint32x8", "get-low", 32, 8, 0, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint32) {
		shuffleGetLoUint32x8(archsimd.LoadUint32x8(x)).Store(got)
	})
	check("GetLoUint32x16", "get-low", 32, 16, 0, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint32) {
		shuffleGetLoUint32x16(archsimd.LoadUint32x16(x)).Store(got)
	})
	check("GetLoUint64x4", "get-low", 64, 4, 0, 2, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint64) {
		shuffleGetLoUint64x4(archsimd.LoadUint64x4(x)).Store(got)
	})
	check("GetLoUint64x8", "get-low", 64, 8, 0, 4, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint64) {
		shuffleGetLoUint64x8(archsimd.LoadUint64x8(x)).Store(got)
	})
	check("InterleaveHiInt16x8", "interleave-high", 16, 8, 8, 8, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []int16) {
		shuffleInterleaveHiInt16x8(archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y)).Store(got)
	})
	check("InterleaveHiInt32x4", "interleave-high", 32, 4, 4, 4, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []int32) {
		shuffleInterleaveHiInt32x4(archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y)).Store(got)
	})
	check("InterleaveHiInt64x2", "interleave-high", 64, 2, 2, 2, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []int64) {
		shuffleInterleaveHiInt64x2(archsimd.LoadInt64x2(x), archsimd.LoadInt64x2(y)).Store(got)
	})
	check("InterleaveHiUint16x8", "interleave-high", 16, 8, 8, 8, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []uint16) {
		shuffleInterleaveHiUint16x8(archsimd.LoadUint16x8(x), archsimd.LoadUint16x8(y)).Store(got)
	})
	check("InterleaveHiUint32x4", "interleave-high", 32, 4, 4, 4, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []uint32) {
		shuffleInterleaveHiUint32x4(archsimd.LoadUint32x4(x), archsimd.LoadUint32x4(y)).Store(got)
	})
	check("InterleaveHiUint64x2", "interleave-high", 64, 2, 2, 2, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []uint64) {
		shuffleInterleaveHiUint64x2(archsimd.LoadUint64x2(x), archsimd.LoadUint64x2(y)).Store(got)
	})
	check("InterleaveHiGroupedInt16x16", "interleave-high-128", 16, 16, 16, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int16) {
		shuffleInterleaveHiGroupedInt16x16(archsimd.LoadInt16x16(x), archsimd.LoadInt16x16(y)).Store(got)
	})
	check("InterleaveHiGroupedInt16x32", "interleave-high-128", 16, 32, 32, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int16) {
		shuffleInterleaveHiGroupedInt16x32(archsimd.LoadInt16x32(x), archsimd.LoadInt16x32(y)).Store(got)
	})
	check("InterleaveHiGroupedInt32x8", "interleave-high-128", 32, 8, 8, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int32) {
		shuffleInterleaveHiGroupedInt32x8(archsimd.LoadInt32x8(x), archsimd.LoadInt32x8(y)).Store(got)
	})
	check("InterleaveHiGroupedInt32x16", "interleave-high-128", 32, 16, 16, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int32) {
		shuffleInterleaveHiGroupedInt32x16(archsimd.LoadInt32x16(x), archsimd.LoadInt32x16(y)).Store(got)
	})
	check("InterleaveHiGroupedInt64x4", "interleave-high-128", 64, 4, 4, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int64) {
		shuffleInterleaveHiGroupedInt64x4(archsimd.LoadInt64x4(x), archsimd.LoadInt64x4(y)).Store(got)
	})
	check("InterleaveHiGroupedInt64x8", "interleave-high-128", 64, 8, 8, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int64) {
		shuffleInterleaveHiGroupedInt64x8(archsimd.LoadInt64x8(x), archsimd.LoadInt64x8(y)).Store(got)
	})
	check("InterleaveHiGroupedUint16x16", "interleave-high-128", 16, 16, 16, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint16) {
		shuffleInterleaveHiGroupedUint16x16(archsimd.LoadUint16x16(x), archsimd.LoadUint16x16(y)).Store(got)
	})
	check("InterleaveHiGroupedUint16x32", "interleave-high-128", 16, 32, 32, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint16) {
		shuffleInterleaveHiGroupedUint16x32(archsimd.LoadUint16x32(x), archsimd.LoadUint16x32(y)).Store(got)
	})
	check("InterleaveHiGroupedUint32x8", "interleave-high-128", 32, 8, 8, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint32) {
		shuffleInterleaveHiGroupedUint32x8(archsimd.LoadUint32x8(x), archsimd.LoadUint32x8(y)).Store(got)
	})
	check("InterleaveHiGroupedUint32x16", "interleave-high-128", 32, 16, 16, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint32) {
		shuffleInterleaveHiGroupedUint32x16(archsimd.LoadUint32x16(x), archsimd.LoadUint32x16(y)).Store(got)
	})
	check("InterleaveHiGroupedUint64x4", "interleave-high-128", 64, 4, 4, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint64) {
		shuffleInterleaveHiGroupedUint64x4(archsimd.LoadUint64x4(x), archsimd.LoadUint64x4(y)).Store(got)
	})
	check("InterleaveHiGroupedUint64x8", "interleave-high-128", 64, 8, 8, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint64) {
		shuffleInterleaveHiGroupedUint64x8(archsimd.LoadUint64x8(x), archsimd.LoadUint64x8(y)).Store(got)
	})
	check("InterleaveLoInt16x8", "interleave-low", 16, 8, 8, 8, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []int16) {
		shuffleInterleaveLoInt16x8(archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y)).Store(got)
	})
	check("InterleaveLoInt32x4", "interleave-low", 32, 4, 4, 4, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []int32) {
		shuffleInterleaveLoInt32x4(archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y)).Store(got)
	})
	check("InterleaveLoInt64x2", "interleave-low", 64, 2, 2, 2, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []int64) {
		shuffleInterleaveLoInt64x2(archsimd.LoadInt64x2(x), archsimd.LoadInt64x2(y)).Store(got)
	})
	check("InterleaveLoUint16x8", "interleave-low", 16, 8, 8, 8, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []uint16) {
		shuffleInterleaveLoUint16x8(archsimd.LoadUint16x8(x), archsimd.LoadUint16x8(y)).Store(got)
	})
	check("InterleaveLoUint32x4", "interleave-low", 32, 4, 4, 4, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []uint32) {
		shuffleInterleaveLoUint32x4(archsimd.LoadUint32x4(x), archsimd.LoadUint32x4(y)).Store(got)
	})
	check("InterleaveLoUint64x2", "interleave-low", 64, 2, 2, 2, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []uint64) {
		shuffleInterleaveLoUint64x2(archsimd.LoadUint64x2(x), archsimd.LoadUint64x2(y)).Store(got)
	})
	check("InterleaveLoGroupedInt16x16", "interleave-low-128", 16, 16, 16, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int16) {
		shuffleInterleaveLoGroupedInt16x16(archsimd.LoadInt16x16(x), archsimd.LoadInt16x16(y)).Store(got)
	})
	check("InterleaveLoGroupedInt16x32", "interleave-low-128", 16, 32, 32, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int16) {
		shuffleInterleaveLoGroupedInt16x32(archsimd.LoadInt16x32(x), archsimd.LoadInt16x32(y)).Store(got)
	})
	check("InterleaveLoGroupedInt32x8", "interleave-low-128", 32, 8, 8, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int32) {
		shuffleInterleaveLoGroupedInt32x8(archsimd.LoadInt32x8(x), archsimd.LoadInt32x8(y)).Store(got)
	})
	check("InterleaveLoGroupedInt32x16", "interleave-low-128", 32, 16, 16, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int32) {
		shuffleInterleaveLoGroupedInt32x16(archsimd.LoadInt32x16(x), archsimd.LoadInt32x16(y)).Store(got)
	})
	check("InterleaveLoGroupedInt64x4", "interleave-low-128", 64, 4, 4, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int64) {
		shuffleInterleaveLoGroupedInt64x4(archsimd.LoadInt64x4(x), archsimd.LoadInt64x4(y)).Store(got)
	})
	check("InterleaveLoGroupedInt64x8", "interleave-low-128", 64, 8, 8, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int64) {
		shuffleInterleaveLoGroupedInt64x8(archsimd.LoadInt64x8(x), archsimd.LoadInt64x8(y)).Store(got)
	})
	check("InterleaveLoGroupedUint16x16", "interleave-low-128", 16, 16, 16, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint16) {
		shuffleInterleaveLoGroupedUint16x16(archsimd.LoadUint16x16(x), archsimd.LoadUint16x16(y)).Store(got)
	})
	check("InterleaveLoGroupedUint16x32", "interleave-low-128", 16, 32, 32, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint16) {
		shuffleInterleaveLoGroupedUint16x32(archsimd.LoadUint16x32(x), archsimd.LoadUint16x32(y)).Store(got)
	})
	check("InterleaveLoGroupedUint32x8", "interleave-low-128", 32, 8, 8, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint32) {
		shuffleInterleaveLoGroupedUint32x8(archsimd.LoadUint32x8(x), archsimd.LoadUint32x8(y)).Store(got)
	})
	check("InterleaveLoGroupedUint32x16", "interleave-low-128", 32, 16, 16, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint32) {
		shuffleInterleaveLoGroupedUint32x16(archsimd.LoadUint32x16(x), archsimd.LoadUint32x16(y)).Store(got)
	})
	check("InterleaveLoGroupedUint64x4", "interleave-low-128", 64, 4, 4, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint64) {
		shuffleInterleaveLoGroupedUint64x4(archsimd.LoadUint64x4(x), archsimd.LoadUint64x4(y)).Store(got)
	})
	check("InterleaveLoGroupedUint64x8", "interleave-low-128", 64, 8, 8, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint64) {
		shuffleInterleaveLoGroupedUint64x8(archsimd.LoadUint64x8(x), archsimd.LoadUint64x8(y)).Store(got)
	})
	check("SetHiFloat32x8", "set-high", 32, 8, 4, 8, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []float32) {
		shuffleSetHiFloat32x8(archsimd.LoadFloat32x8(x), archsimd.LoadFloat32x4(y)).Store(got)
	})
	check("SetHiFloat32x16", "set-high", 32, 16, 8, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []float32) {
		shuffleSetHiFloat32x16(archsimd.LoadFloat32x16(x), archsimd.LoadFloat32x8(y)).Store(got)
	})
	check("SetHiFloat64x4", "set-high", 64, 4, 2, 4, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []float64) {
		shuffleSetHiFloat64x4(archsimd.LoadFloat64x4(x), archsimd.LoadFloat64x2(y)).Store(got)
	})
	check("SetHiFloat64x8", "set-high", 64, 8, 4, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []float64) {
		shuffleSetHiFloat64x8(archsimd.LoadFloat64x8(x), archsimd.LoadFloat64x4(y)).Store(got)
	})
	check("SetHiInt8x32", "set-high", 8, 32, 16, 32, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int8) {
		shuffleSetHiInt8x32(archsimd.LoadInt8x32(x), archsimd.LoadInt8x16(y)).Store(got)
	})
	check("SetHiInt8x64", "set-high", 8, 64, 32, 64, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int8) {
		shuffleSetHiInt8x64(archsimd.LoadInt8x64(x), archsimd.LoadInt8x32(y)).Store(got)
	})
	check("SetHiInt16x16", "set-high", 16, 16, 8, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int16) {
		shuffleSetHiInt16x16(archsimd.LoadInt16x16(x), archsimd.LoadInt16x8(y)).Store(got)
	})
	check("SetHiInt16x32", "set-high", 16, 32, 16, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int16) {
		shuffleSetHiInt16x32(archsimd.LoadInt16x32(x), archsimd.LoadInt16x16(y)).Store(got)
	})
	check("SetHiInt32x8", "set-high", 32, 8, 4, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int32) {
		shuffleSetHiInt32x8(archsimd.LoadInt32x8(x), archsimd.LoadInt32x4(y)).Store(got)
	})
	check("SetHiInt32x16", "set-high", 32, 16, 8, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int32) {
		shuffleSetHiInt32x16(archsimd.LoadInt32x16(x), archsimd.LoadInt32x8(y)).Store(got)
	})
	check("SetHiInt64x4", "set-high", 64, 4, 2, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int64) {
		shuffleSetHiInt64x4(archsimd.LoadInt64x4(x), archsimd.LoadInt64x2(y)).Store(got)
	})
	check("SetHiInt64x8", "set-high", 64, 8, 4, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int64) {
		shuffleSetHiInt64x8(archsimd.LoadInt64x8(x), archsimd.LoadInt64x4(y)).Store(got)
	})
	check("SetHiUint8x32", "set-high", 8, 32, 16, 32, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint8) {
		shuffleSetHiUint8x32(archsimd.LoadUint8x32(x), archsimd.LoadUint8x16(y)).Store(got)
	})
	check("SetHiUint8x64", "set-high", 8, 64, 32, 64, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint8) {
		shuffleSetHiUint8x64(archsimd.LoadUint8x64(x), archsimd.LoadUint8x32(y)).Store(got)
	})
	check("SetHiUint16x16", "set-high", 16, 16, 8, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint16) {
		shuffleSetHiUint16x16(archsimd.LoadUint16x16(x), archsimd.LoadUint16x8(y)).Store(got)
	})
	check("SetHiUint16x32", "set-high", 16, 32, 16, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint16) {
		shuffleSetHiUint16x32(archsimd.LoadUint16x32(x), archsimd.LoadUint16x16(y)).Store(got)
	})
	check("SetHiUint32x8", "set-high", 32, 8, 4, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint32) {
		shuffleSetHiUint32x8(archsimd.LoadUint32x8(x), archsimd.LoadUint32x4(y)).Store(got)
	})
	check("SetHiUint32x16", "set-high", 32, 16, 8, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint32) {
		shuffleSetHiUint32x16(archsimd.LoadUint32x16(x), archsimd.LoadUint32x8(y)).Store(got)
	})
	check("SetHiUint64x4", "set-high", 64, 4, 2, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint64) {
		shuffleSetHiUint64x4(archsimd.LoadUint64x4(x), archsimd.LoadUint64x2(y)).Store(got)
	})
	check("SetHiUint64x8", "set-high", 64, 8, 4, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint64) {
		shuffleSetHiUint64x8(archsimd.LoadUint64x8(x), archsimd.LoadUint64x4(y)).Store(got)
	})
	check("SetLoFloat32x8", "set-low", 32, 8, 4, 8, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []float32) {
		shuffleSetLoFloat32x8(archsimd.LoadFloat32x8(x), archsimd.LoadFloat32x4(y)).Store(got)
	})
	check("SetLoFloat32x16", "set-low", 32, 16, 8, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []float32) {
		shuffleSetLoFloat32x16(archsimd.LoadFloat32x16(x), archsimd.LoadFloat32x8(y)).Store(got)
	})
	check("SetLoFloat64x4", "set-low", 64, 4, 2, 4, archsimd.X86.AVX(), archsimd.X86.AVX(), func(x, y, got []float64) {
		shuffleSetLoFloat64x4(archsimd.LoadFloat64x4(x), archsimd.LoadFloat64x2(y)).Store(got)
	})
	check("SetLoFloat64x8", "set-low", 64, 8, 4, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []float64) {
		shuffleSetLoFloat64x8(archsimd.LoadFloat64x8(x), archsimd.LoadFloat64x4(y)).Store(got)
	})
	check("SetLoInt8x32", "set-low", 8, 32, 16, 32, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int8) {
		shuffleSetLoInt8x32(archsimd.LoadInt8x32(x), archsimd.LoadInt8x16(y)).Store(got)
	})
	check("SetLoInt8x64", "set-low", 8, 64, 32, 64, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int8) {
		shuffleSetLoInt8x64(archsimd.LoadInt8x64(x), archsimd.LoadInt8x32(y)).Store(got)
	})
	check("SetLoInt16x16", "set-low", 16, 16, 8, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int16) {
		shuffleSetLoInt16x16(archsimd.LoadInt16x16(x), archsimd.LoadInt16x8(y)).Store(got)
	})
	check("SetLoInt16x32", "set-low", 16, 32, 16, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int16) {
		shuffleSetLoInt16x32(archsimd.LoadInt16x32(x), archsimd.LoadInt16x16(y)).Store(got)
	})
	check("SetLoInt32x8", "set-low", 32, 8, 4, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int32) {
		shuffleSetLoInt32x8(archsimd.LoadInt32x8(x), archsimd.LoadInt32x4(y)).Store(got)
	})
	check("SetLoInt32x16", "set-low", 32, 16, 8, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int32) {
		shuffleSetLoInt32x16(archsimd.LoadInt32x16(x), archsimd.LoadInt32x8(y)).Store(got)
	})
	check("SetLoInt64x4", "set-low", 64, 4, 2, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int64) {
		shuffleSetLoInt64x4(archsimd.LoadInt64x4(x), archsimd.LoadInt64x2(y)).Store(got)
	})
	check("SetLoInt64x8", "set-low", 64, 8, 4, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int64) {
		shuffleSetLoInt64x8(archsimd.LoadInt64x8(x), archsimd.LoadInt64x4(y)).Store(got)
	})
	check("SetLoUint8x32", "set-low", 8, 32, 16, 32, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint8) {
		shuffleSetLoUint8x32(archsimd.LoadUint8x32(x), archsimd.LoadUint8x16(y)).Store(got)
	})
	check("SetLoUint8x64", "set-low", 8, 64, 32, 64, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint8) {
		shuffleSetLoUint8x64(archsimd.LoadUint8x64(x), archsimd.LoadUint8x32(y)).Store(got)
	})
	check("SetLoUint16x16", "set-low", 16, 16, 8, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint16) {
		shuffleSetLoUint16x16(archsimd.LoadUint16x16(x), archsimd.LoadUint16x8(y)).Store(got)
	})
	check("SetLoUint16x32", "set-low", 16, 32, 16, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint16) {
		shuffleSetLoUint16x32(archsimd.LoadUint16x32(x), archsimd.LoadUint16x16(y)).Store(got)
	})
	check("SetLoUint32x8", "set-low", 32, 8, 4, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint32) {
		shuffleSetLoUint32x8(archsimd.LoadUint32x8(x), archsimd.LoadUint32x4(y)).Store(got)
	})
	check("SetLoUint32x16", "set-low", 32, 16, 8, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint32) {
		shuffleSetLoUint32x16(archsimd.LoadUint32x16(x), archsimd.LoadUint32x8(y)).Store(got)
	})
	check("SetLoUint64x4", "set-low", 64, 4, 2, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint64) {
		shuffleSetLoUint64x4(archsimd.LoadUint64x4(x), archsimd.LoadUint64x2(y)).Store(got)
	})
	check("SetLoUint64x8", "set-low", 64, 8, 4, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint64) {
		shuffleSetLoUint64x8(archsimd.LoadUint64x8(x), archsimd.LoadUint64x4(y)).Store(got)
	})
	check("broadcast1To2Float64x2", "broadcast-low", 64, 2, 0, 2, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []float64) {
		shufflebroadcast1To2Float64x2(archsimd.LoadFloat64x2(x)).Store(got)
	})
	check("broadcast1To2Int64x2", "broadcast-low", 64, 2, 0, 2, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int64) {
		shufflebroadcast1To2Int64x2(archsimd.LoadInt64x2(x)).Store(got)
	})
	check("broadcast1To2Uint64x2", "broadcast-low", 64, 2, 0, 2, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint64) {
		shufflebroadcast1To2Uint64x2(archsimd.LoadUint64x2(x)).Store(got)
	})
	check("broadcast1To4Float32x4", "broadcast-low", 32, 4, 0, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []float32) {
		shufflebroadcast1To4Float32x4(archsimd.LoadFloat32x4(x)).Store(got)
	})
	check("broadcast1To4Float64x2", "broadcast-low", 64, 2, 0, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []float64) {
		shufflebroadcast1To4Float64x2(archsimd.LoadFloat64x2(x)).Store(got)
	})
	check("broadcast1To4Int32x4", "broadcast-low", 32, 4, 0, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int32) {
		shufflebroadcast1To4Int32x4(archsimd.LoadInt32x4(x)).Store(got)
	})
	check("broadcast1To4Int64x2", "broadcast-low", 64, 2, 0, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int64) {
		shufflebroadcast1To4Int64x2(archsimd.LoadInt64x2(x)).Store(got)
	})
	check("broadcast1To4Uint32x4", "broadcast-low", 32, 4, 0, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint32) {
		shufflebroadcast1To4Uint32x4(archsimd.LoadUint32x4(x)).Store(got)
	})
	check("broadcast1To4Uint64x2", "broadcast-low", 64, 2, 0, 4, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint64) {
		shufflebroadcast1To4Uint64x2(archsimd.LoadUint64x2(x)).Store(got)
	})
	check("broadcast1To8Float32x4", "broadcast-low", 32, 4, 0, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []float32) {
		shufflebroadcast1To8Float32x4(archsimd.LoadFloat32x4(x)).Store(got)
	})
	check("broadcast1To8Float64x2", "broadcast-low", 64, 2, 0, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []float64) {
		shufflebroadcast1To8Float64x2(archsimd.LoadFloat64x2(x)).Store(got)
	})
	check("broadcast1To8Int16x8", "broadcast-low", 16, 8, 0, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int16) {
		shufflebroadcast1To8Int16x8(archsimd.LoadInt16x8(x)).Store(got)
	})
	check("broadcast1To8Int32x4", "broadcast-low", 32, 4, 0, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int32) {
		shufflebroadcast1To8Int32x4(archsimd.LoadInt32x4(x)).Store(got)
	})
	check("broadcast1To8Int64x2", "broadcast-low", 64, 2, 0, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int64) {
		shufflebroadcast1To8Int64x2(archsimd.LoadInt64x2(x)).Store(got)
	})
	check("broadcast1To8Uint16x8", "broadcast-low", 16, 8, 0, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint16) {
		shufflebroadcast1To8Uint16x8(archsimd.LoadUint16x8(x)).Store(got)
	})
	check("broadcast1To8Uint32x4", "broadcast-low", 32, 4, 0, 8, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint32) {
		shufflebroadcast1To8Uint32x4(archsimd.LoadUint32x4(x)).Store(got)
	})
	check("broadcast1To8Uint64x2", "broadcast-low", 64, 2, 0, 8, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint64) {
		shufflebroadcast1To8Uint64x2(archsimd.LoadUint64x2(x)).Store(got)
	})
	check("broadcast1To16Float32x4", "broadcast-low", 32, 4, 0, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []float32) {
		shufflebroadcast1To16Float32x4(archsimd.LoadFloat32x4(x)).Store(got)
	})
	check("broadcast1To16Int8x16", "broadcast-low", 8, 16, 0, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int8) {
		shufflebroadcast1To16Int8x16(archsimd.LoadInt8x16(x)).Store(got)
	})
	check("broadcast1To16Int16x8", "broadcast-low", 16, 8, 0, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int16) {
		shufflebroadcast1To16Int16x8(archsimd.LoadInt16x8(x)).Store(got)
	})
	check("broadcast1To16Int32x4", "broadcast-low", 32, 4, 0, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int32) {
		shufflebroadcast1To16Int32x4(archsimd.LoadInt32x4(x)).Store(got)
	})
	check("broadcast1To16Uint8x16", "broadcast-low", 8, 16, 0, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint8) {
		shufflebroadcast1To16Uint8x16(archsimd.LoadUint8x16(x)).Store(got)
	})
	check("broadcast1To16Uint16x8", "broadcast-low", 16, 8, 0, 16, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint16) {
		shufflebroadcast1To16Uint16x8(archsimd.LoadUint16x8(x)).Store(got)
	})
	check("broadcast1To16Uint32x4", "broadcast-low", 32, 4, 0, 16, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint32) {
		shufflebroadcast1To16Uint32x4(archsimd.LoadUint32x4(x)).Store(got)
	})
	check("broadcast1To32Int8x16", "broadcast-low", 8, 16, 0, 32, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []int8) {
		shufflebroadcast1To32Int8x16(archsimd.LoadInt8x16(x)).Store(got)
	})
	check("broadcast1To32Int16x8", "broadcast-low", 16, 8, 0, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int16) {
		shufflebroadcast1To32Int16x8(archsimd.LoadInt16x8(x)).Store(got)
	})
	check("broadcast1To32Uint8x16", "broadcast-low", 8, 16, 0, 32, archsimd.X86.AVX(), archsimd.X86.AVX2(), func(x, y, got []uint8) {
		shufflebroadcast1To32Uint8x16(archsimd.LoadUint8x16(x)).Store(got)
	})
	check("broadcast1To32Uint16x8", "broadcast-low", 16, 8, 0, 32, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint16) {
		shufflebroadcast1To32Uint16x8(archsimd.LoadUint16x8(x)).Store(got)
	})
	check("broadcast1To64Int8x16", "broadcast-low", 8, 16, 0, 64, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []int8) {
		shufflebroadcast1To64Int8x16(archsimd.LoadInt8x16(x)).Store(got)
	})
	check("broadcast1To64Uint8x16", "broadcast-low", 8, 16, 0, 64, archsimd.X86.AVX512(), archsimd.X86.AVX512(), func(x, y, got []uint8) {
		shufflebroadcast1To64Uint8x16(archsimd.LoadUint8x16(x)).Store(got)
	})
}
