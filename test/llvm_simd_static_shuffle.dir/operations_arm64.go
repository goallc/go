// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func shuffleConcatEvenInt8x16(x archsimd.Int8x16, y archsimd.Int8x16) archsimd.Int8x16 {
	return x.ConcatEven(y)
}

//go:noinline
func shuffleConcatEvenInt16x8(x archsimd.Int16x8, y archsimd.Int16x8) archsimd.Int16x8 {
	return x.ConcatEven(y)
}

//go:noinline
func shuffleConcatEvenInt32x4(x archsimd.Int32x4, y archsimd.Int32x4) archsimd.Int32x4 {
	return x.ConcatEven(y)
}

//go:noinline
func shuffleConcatEvenInt64x2(x archsimd.Int64x2, y archsimd.Int64x2) archsimd.Int64x2 {
	return x.ConcatEven(y)
}

//go:noinline
func shuffleConcatEvenUint8x16(x archsimd.Uint8x16, y archsimd.Uint8x16) archsimd.Uint8x16 {
	return x.ConcatEven(y)
}

//go:noinline
func shuffleConcatEvenUint16x8(x archsimd.Uint16x8, y archsimd.Uint16x8) archsimd.Uint16x8 {
	return x.ConcatEven(y)
}

//go:noinline
func shuffleConcatEvenUint32x4(x archsimd.Uint32x4, y archsimd.Uint32x4) archsimd.Uint32x4 {
	return x.ConcatEven(y)
}

//go:noinline
func shuffleConcatEvenUint64x2(x archsimd.Uint64x2, y archsimd.Uint64x2) archsimd.Uint64x2 {
	return x.ConcatEven(y)
}

//go:noinline
func shuffleConcatOddInt8x16(x archsimd.Int8x16, y archsimd.Int8x16) archsimd.Int8x16 {
	return x.ConcatOdd(y)
}

//go:noinline
func shuffleConcatOddInt16x8(x archsimd.Int16x8, y archsimd.Int16x8) archsimd.Int16x8 {
	return x.ConcatOdd(y)
}

//go:noinline
func shuffleConcatOddInt32x4(x archsimd.Int32x4, y archsimd.Int32x4) archsimd.Int32x4 {
	return x.ConcatOdd(y)
}

//go:noinline
func shuffleConcatOddInt64x2(x archsimd.Int64x2, y archsimd.Int64x2) archsimd.Int64x2 {
	return x.ConcatOdd(y)
}

//go:noinline
func shuffleConcatOddUint8x16(x archsimd.Uint8x16, y archsimd.Uint8x16) archsimd.Uint8x16 {
	return x.ConcatOdd(y)
}

//go:noinline
func shuffleConcatOddUint16x8(x archsimd.Uint16x8, y archsimd.Uint16x8) archsimd.Uint16x8 {
	return x.ConcatOdd(y)
}

//go:noinline
func shuffleConcatOddUint32x4(x archsimd.Uint32x4, y archsimd.Uint32x4) archsimd.Uint32x4 {
	return x.ConcatOdd(y)
}

//go:noinline
func shuffleConcatOddUint64x2(x archsimd.Uint64x2, y archsimd.Uint64x2) archsimd.Uint64x2 {
	return x.ConcatOdd(y)
}

//go:noinline
func shuffleInterleaveEvenInt8x16(x archsimd.Int8x16, y archsimd.Int8x16) archsimd.Int8x16 {
	return x.InterleaveEven(y)
}

//go:noinline
func shuffleInterleaveEvenInt16x8(x archsimd.Int16x8, y archsimd.Int16x8) archsimd.Int16x8 {
	return x.InterleaveEven(y)
}

//go:noinline
func shuffleInterleaveEvenInt32x4(x archsimd.Int32x4, y archsimd.Int32x4) archsimd.Int32x4 {
	return x.InterleaveEven(y)
}

//go:noinline
func shuffleInterleaveEvenInt64x2(x archsimd.Int64x2, y archsimd.Int64x2) archsimd.Int64x2 {
	return x.InterleaveEven(y)
}

//go:noinline
func shuffleInterleaveEvenUint8x16(x archsimd.Uint8x16, y archsimd.Uint8x16) archsimd.Uint8x16 {
	return x.InterleaveEven(y)
}

//go:noinline
func shuffleInterleaveEvenUint16x8(x archsimd.Uint16x8, y archsimd.Uint16x8) archsimd.Uint16x8 {
	return x.InterleaveEven(y)
}

//go:noinline
func shuffleInterleaveEvenUint32x4(x archsimd.Uint32x4, y archsimd.Uint32x4) archsimd.Uint32x4 {
	return x.InterleaveEven(y)
}

//go:noinline
func shuffleInterleaveEvenUint64x2(x archsimd.Uint64x2, y archsimd.Uint64x2) archsimd.Uint64x2 {
	return x.InterleaveEven(y)
}

//go:noinline
func shuffleInterleaveHiInt8x16(x archsimd.Int8x16, y archsimd.Int8x16) archsimd.Int8x16 {
	return x.InterleaveHi(y)
}

//go:noinline
func shuffleInterleaveHiInt16x8(x archsimd.Int16x8, y archsimd.Int16x8) archsimd.Int16x8 {
	return x.InterleaveHi(y)
}

//go:noinline
func shuffleInterleaveHiInt32x4(x archsimd.Int32x4, y archsimd.Int32x4) archsimd.Int32x4 {
	return x.InterleaveHi(y)
}

//go:noinline
func shuffleInterleaveHiInt64x2(x archsimd.Int64x2, y archsimd.Int64x2) archsimd.Int64x2 {
	return x.InterleaveHi(y)
}

//go:noinline
func shuffleInterleaveHiUint8x16(x archsimd.Uint8x16, y archsimd.Uint8x16) archsimd.Uint8x16 {
	return x.InterleaveHi(y)
}

//go:noinline
func shuffleInterleaveHiUint16x8(x archsimd.Uint16x8, y archsimd.Uint16x8) archsimd.Uint16x8 {
	return x.InterleaveHi(y)
}

//go:noinline
func shuffleInterleaveHiUint32x4(x archsimd.Uint32x4, y archsimd.Uint32x4) archsimd.Uint32x4 {
	return x.InterleaveHi(y)
}

//go:noinline
func shuffleInterleaveHiUint64x2(x archsimd.Uint64x2, y archsimd.Uint64x2) archsimd.Uint64x2 {
	return x.InterleaveHi(y)
}

//go:noinline
func shuffleInterleaveLoInt8x16(x archsimd.Int8x16, y archsimd.Int8x16) archsimd.Int8x16 {
	return x.InterleaveLo(y)
}

//go:noinline
func shuffleInterleaveLoInt16x8(x archsimd.Int16x8, y archsimd.Int16x8) archsimd.Int16x8 {
	return x.InterleaveLo(y)
}

//go:noinline
func shuffleInterleaveLoInt32x4(x archsimd.Int32x4, y archsimd.Int32x4) archsimd.Int32x4 {
	return x.InterleaveLo(y)
}

//go:noinline
func shuffleInterleaveLoInt64x2(x archsimd.Int64x2, y archsimd.Int64x2) archsimd.Int64x2 {
	return x.InterleaveLo(y)
}

//go:noinline
func shuffleInterleaveLoUint8x16(x archsimd.Uint8x16, y archsimd.Uint8x16) archsimd.Uint8x16 {
	return x.InterleaveLo(y)
}

//go:noinline
func shuffleInterleaveLoUint16x8(x archsimd.Uint16x8, y archsimd.Uint16x8) archsimd.Uint16x8 {
	return x.InterleaveLo(y)
}

//go:noinline
func shuffleInterleaveLoUint32x4(x archsimd.Uint32x4, y archsimd.Uint32x4) archsimd.Uint32x4 {
	return x.InterleaveLo(y)
}

//go:noinline
func shuffleInterleaveLoUint64x2(x archsimd.Uint64x2, y archsimd.Uint64x2) archsimd.Uint64x2 {
	return x.InterleaveLo(y)
}

//go:noinline
func shuffleInterleaveOddInt8x16(x archsimd.Int8x16, y archsimd.Int8x16) archsimd.Int8x16 {
	return x.InterleaveOdd(y)
}

//go:noinline
func shuffleInterleaveOddInt16x8(x archsimd.Int16x8, y archsimd.Int16x8) archsimd.Int16x8 {
	return x.InterleaveOdd(y)
}

//go:noinline
func shuffleInterleaveOddInt32x4(x archsimd.Int32x4, y archsimd.Int32x4) archsimd.Int32x4 {
	return x.InterleaveOdd(y)
}

//go:noinline
func shuffleInterleaveOddInt64x2(x archsimd.Int64x2, y archsimd.Int64x2) archsimd.Int64x2 {
	return x.InterleaveOdd(y)
}

//go:noinline
func shuffleInterleaveOddUint8x16(x archsimd.Uint8x16, y archsimd.Uint8x16) archsimd.Uint8x16 {
	return x.InterleaveOdd(y)
}

//go:noinline
func shuffleInterleaveOddUint16x8(x archsimd.Uint16x8, y archsimd.Uint16x8) archsimd.Uint16x8 {
	return x.InterleaveOdd(y)
}

//go:noinline
func shuffleInterleaveOddUint32x4(x archsimd.Uint32x4, y archsimd.Uint32x4) archsimd.Uint32x4 {
	return x.InterleaveOdd(y)
}

//go:noinline
func shuffleInterleaveOddUint64x2(x archsimd.Uint64x2, y archsimd.Uint64x2) archsimd.Uint64x2 {
	return x.InterleaveOdd(y)
}

//go:noinline
func shufflebroadcast1To2Float64x2(x archsimd.Float64x2) archsimd.Float64x2 {
	return archsimd.BroadcastFloat64x2(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To2Int64x2(x archsimd.Int64x2) archsimd.Int64x2 {
	return archsimd.BroadcastInt64x2(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To2Uint64x2(x archsimd.Uint64x2) archsimd.Uint64x2 {
	return archsimd.BroadcastUint64x2(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To4Float32x4(x archsimd.Float32x4) archsimd.Float32x4 {
	return archsimd.BroadcastFloat32x4(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To4Int32x4(x archsimd.Int32x4) archsimd.Int32x4 {
	return archsimd.BroadcastInt32x4(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To4Uint32x4(x archsimd.Uint32x4) archsimd.Uint32x4 {
	return archsimd.BroadcastUint32x4(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To8Int16x8(x archsimd.Int16x8) archsimd.Int16x8 {
	return archsimd.BroadcastInt16x8(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To8Uint16x8(x archsimd.Uint16x8) archsimd.Uint16x8 {
	return archsimd.BroadcastUint16x8(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To16Int8x16(x archsimd.Int8x16) archsimd.Int8x16 {
	return archsimd.BroadcastInt8x16(x.GetElem(0))
}

//go:noinline
func shufflebroadcast1To16Uint8x16(x archsimd.Uint8x16) archsimd.Uint8x16 {
	return archsimd.BroadcastUint8x16(x.GetElem(0))
}

func checkArch() {
	check("ConcatEvenInt8x16", "concat-even", 8, 16, 16, 16, true, true, func(x, y, got []int8) {
		shuffleConcatEvenInt8x16(archsimd.LoadInt8x16(x), archsimd.LoadInt8x16(y)).Store(got)
	})
	check("ConcatEvenInt16x8", "concat-even", 16, 8, 8, 8, true, true, func(x, y, got []int16) {
		shuffleConcatEvenInt16x8(archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y)).Store(got)
	})
	check("ConcatEvenInt32x4", "concat-even", 32, 4, 4, 4, true, true, func(x, y, got []int32) {
		shuffleConcatEvenInt32x4(archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y)).Store(got)
	})
	check("ConcatEvenInt64x2", "concat-even", 64, 2, 2, 2, true, true, func(x, y, got []int64) {
		shuffleConcatEvenInt64x2(archsimd.LoadInt64x2(x), archsimd.LoadInt64x2(y)).Store(got)
	})
	check("ConcatEvenUint8x16", "concat-even", 8, 16, 16, 16, true, true, func(x, y, got []uint8) {
		shuffleConcatEvenUint8x16(archsimd.LoadUint8x16(x), archsimd.LoadUint8x16(y)).Store(got)
	})
	check("ConcatEvenUint16x8", "concat-even", 16, 8, 8, 8, true, true, func(x, y, got []uint16) {
		shuffleConcatEvenUint16x8(archsimd.LoadUint16x8(x), archsimd.LoadUint16x8(y)).Store(got)
	})
	check("ConcatEvenUint32x4", "concat-even", 32, 4, 4, 4, true, true, func(x, y, got []uint32) {
		shuffleConcatEvenUint32x4(archsimd.LoadUint32x4(x), archsimd.LoadUint32x4(y)).Store(got)
	})
	check("ConcatEvenUint64x2", "concat-even", 64, 2, 2, 2, true, true, func(x, y, got []uint64) {
		shuffleConcatEvenUint64x2(archsimd.LoadUint64x2(x), archsimd.LoadUint64x2(y)).Store(got)
	})
	check("ConcatOddInt8x16", "concat-odd", 8, 16, 16, 16, true, true, func(x, y, got []int8) {
		shuffleConcatOddInt8x16(archsimd.LoadInt8x16(x), archsimd.LoadInt8x16(y)).Store(got)
	})
	check("ConcatOddInt16x8", "concat-odd", 16, 8, 8, 8, true, true, func(x, y, got []int16) {
		shuffleConcatOddInt16x8(archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y)).Store(got)
	})
	check("ConcatOddInt32x4", "concat-odd", 32, 4, 4, 4, true, true, func(x, y, got []int32) {
		shuffleConcatOddInt32x4(archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y)).Store(got)
	})
	check("ConcatOddInt64x2", "concat-odd", 64, 2, 2, 2, true, true, func(x, y, got []int64) {
		shuffleConcatOddInt64x2(archsimd.LoadInt64x2(x), archsimd.LoadInt64x2(y)).Store(got)
	})
	check("ConcatOddUint8x16", "concat-odd", 8, 16, 16, 16, true, true, func(x, y, got []uint8) {
		shuffleConcatOddUint8x16(archsimd.LoadUint8x16(x), archsimd.LoadUint8x16(y)).Store(got)
	})
	check("ConcatOddUint16x8", "concat-odd", 16, 8, 8, 8, true, true, func(x, y, got []uint16) {
		shuffleConcatOddUint16x8(archsimd.LoadUint16x8(x), archsimd.LoadUint16x8(y)).Store(got)
	})
	check("ConcatOddUint32x4", "concat-odd", 32, 4, 4, 4, true, true, func(x, y, got []uint32) {
		shuffleConcatOddUint32x4(archsimd.LoadUint32x4(x), archsimd.LoadUint32x4(y)).Store(got)
	})
	check("ConcatOddUint64x2", "concat-odd", 64, 2, 2, 2, true, true, func(x, y, got []uint64) {
		shuffleConcatOddUint64x2(archsimd.LoadUint64x2(x), archsimd.LoadUint64x2(y)).Store(got)
	})
	check("InterleaveEvenInt8x16", "interleave-even", 8, 16, 16, 16, true, true, func(x, y, got []int8) {
		shuffleInterleaveEvenInt8x16(archsimd.LoadInt8x16(x), archsimd.LoadInt8x16(y)).Store(got)
	})
	check("InterleaveEvenInt16x8", "interleave-even", 16, 8, 8, 8, true, true, func(x, y, got []int16) {
		shuffleInterleaveEvenInt16x8(archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y)).Store(got)
	})
	check("InterleaveEvenInt32x4", "interleave-even", 32, 4, 4, 4, true, true, func(x, y, got []int32) {
		shuffleInterleaveEvenInt32x4(archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y)).Store(got)
	})
	check("InterleaveEvenInt64x2", "interleave-even", 64, 2, 2, 2, true, true, func(x, y, got []int64) {
		shuffleInterleaveEvenInt64x2(archsimd.LoadInt64x2(x), archsimd.LoadInt64x2(y)).Store(got)
	})
	check("InterleaveEvenUint8x16", "interleave-even", 8, 16, 16, 16, true, true, func(x, y, got []uint8) {
		shuffleInterleaveEvenUint8x16(archsimd.LoadUint8x16(x), archsimd.LoadUint8x16(y)).Store(got)
	})
	check("InterleaveEvenUint16x8", "interleave-even", 16, 8, 8, 8, true, true, func(x, y, got []uint16) {
		shuffleInterleaveEvenUint16x8(archsimd.LoadUint16x8(x), archsimd.LoadUint16x8(y)).Store(got)
	})
	check("InterleaveEvenUint32x4", "interleave-even", 32, 4, 4, 4, true, true, func(x, y, got []uint32) {
		shuffleInterleaveEvenUint32x4(archsimd.LoadUint32x4(x), archsimd.LoadUint32x4(y)).Store(got)
	})
	check("InterleaveEvenUint64x2", "interleave-even", 64, 2, 2, 2, true, true, func(x, y, got []uint64) {
		shuffleInterleaveEvenUint64x2(archsimd.LoadUint64x2(x), archsimd.LoadUint64x2(y)).Store(got)
	})
	check("InterleaveHiInt8x16", "interleave-high", 8, 16, 16, 16, true, true, func(x, y, got []int8) {
		shuffleInterleaveHiInt8x16(archsimd.LoadInt8x16(x), archsimd.LoadInt8x16(y)).Store(got)
	})
	check("InterleaveHiInt16x8", "interleave-high", 16, 8, 8, 8, true, true, func(x, y, got []int16) {
		shuffleInterleaveHiInt16x8(archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y)).Store(got)
	})
	check("InterleaveHiInt32x4", "interleave-high", 32, 4, 4, 4, true, true, func(x, y, got []int32) {
		shuffleInterleaveHiInt32x4(archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y)).Store(got)
	})
	check("InterleaveHiInt64x2", "interleave-high", 64, 2, 2, 2, true, true, func(x, y, got []int64) {
		shuffleInterleaveHiInt64x2(archsimd.LoadInt64x2(x), archsimd.LoadInt64x2(y)).Store(got)
	})
	check("InterleaveHiUint8x16", "interleave-high", 8, 16, 16, 16, true, true, func(x, y, got []uint8) {
		shuffleInterleaveHiUint8x16(archsimd.LoadUint8x16(x), archsimd.LoadUint8x16(y)).Store(got)
	})
	check("InterleaveHiUint16x8", "interleave-high", 16, 8, 8, 8, true, true, func(x, y, got []uint16) {
		shuffleInterleaveHiUint16x8(archsimd.LoadUint16x8(x), archsimd.LoadUint16x8(y)).Store(got)
	})
	check("InterleaveHiUint32x4", "interleave-high", 32, 4, 4, 4, true, true, func(x, y, got []uint32) {
		shuffleInterleaveHiUint32x4(archsimd.LoadUint32x4(x), archsimd.LoadUint32x4(y)).Store(got)
	})
	check("InterleaveHiUint64x2", "interleave-high", 64, 2, 2, 2, true, true, func(x, y, got []uint64) {
		shuffleInterleaveHiUint64x2(archsimd.LoadUint64x2(x), archsimd.LoadUint64x2(y)).Store(got)
	})
	check("InterleaveLoInt8x16", "interleave-low", 8, 16, 16, 16, true, true, func(x, y, got []int8) {
		shuffleInterleaveLoInt8x16(archsimd.LoadInt8x16(x), archsimd.LoadInt8x16(y)).Store(got)
	})
	check("InterleaveLoInt16x8", "interleave-low", 16, 8, 8, 8, true, true, func(x, y, got []int16) {
		shuffleInterleaveLoInt16x8(archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y)).Store(got)
	})
	check("InterleaveLoInt32x4", "interleave-low", 32, 4, 4, 4, true, true, func(x, y, got []int32) {
		shuffleInterleaveLoInt32x4(archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y)).Store(got)
	})
	check("InterleaveLoInt64x2", "interleave-low", 64, 2, 2, 2, true, true, func(x, y, got []int64) {
		shuffleInterleaveLoInt64x2(archsimd.LoadInt64x2(x), archsimd.LoadInt64x2(y)).Store(got)
	})
	check("InterleaveLoUint8x16", "interleave-low", 8, 16, 16, 16, true, true, func(x, y, got []uint8) {
		shuffleInterleaveLoUint8x16(archsimd.LoadUint8x16(x), archsimd.LoadUint8x16(y)).Store(got)
	})
	check("InterleaveLoUint16x8", "interleave-low", 16, 8, 8, 8, true, true, func(x, y, got []uint16) {
		shuffleInterleaveLoUint16x8(archsimd.LoadUint16x8(x), archsimd.LoadUint16x8(y)).Store(got)
	})
	check("InterleaveLoUint32x4", "interleave-low", 32, 4, 4, 4, true, true, func(x, y, got []uint32) {
		shuffleInterleaveLoUint32x4(archsimd.LoadUint32x4(x), archsimd.LoadUint32x4(y)).Store(got)
	})
	check("InterleaveLoUint64x2", "interleave-low", 64, 2, 2, 2, true, true, func(x, y, got []uint64) {
		shuffleInterleaveLoUint64x2(archsimd.LoadUint64x2(x), archsimd.LoadUint64x2(y)).Store(got)
	})
	check("InterleaveOddInt8x16", "interleave-odd", 8, 16, 16, 16, true, true, func(x, y, got []int8) {
		shuffleInterleaveOddInt8x16(archsimd.LoadInt8x16(x), archsimd.LoadInt8x16(y)).Store(got)
	})
	check("InterleaveOddInt16x8", "interleave-odd", 16, 8, 8, 8, true, true, func(x, y, got []int16) {
		shuffleInterleaveOddInt16x8(archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y)).Store(got)
	})
	check("InterleaveOddInt32x4", "interleave-odd", 32, 4, 4, 4, true, true, func(x, y, got []int32) {
		shuffleInterleaveOddInt32x4(archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y)).Store(got)
	})
	check("InterleaveOddInt64x2", "interleave-odd", 64, 2, 2, 2, true, true, func(x, y, got []int64) {
		shuffleInterleaveOddInt64x2(archsimd.LoadInt64x2(x), archsimd.LoadInt64x2(y)).Store(got)
	})
	check("InterleaveOddUint8x16", "interleave-odd", 8, 16, 16, 16, true, true, func(x, y, got []uint8) {
		shuffleInterleaveOddUint8x16(archsimd.LoadUint8x16(x), archsimd.LoadUint8x16(y)).Store(got)
	})
	check("InterleaveOddUint16x8", "interleave-odd", 16, 8, 8, 8, true, true, func(x, y, got []uint16) {
		shuffleInterleaveOddUint16x8(archsimd.LoadUint16x8(x), archsimd.LoadUint16x8(y)).Store(got)
	})
	check("InterleaveOddUint32x4", "interleave-odd", 32, 4, 4, 4, true, true, func(x, y, got []uint32) {
		shuffleInterleaveOddUint32x4(archsimd.LoadUint32x4(x), archsimd.LoadUint32x4(y)).Store(got)
	})
	check("InterleaveOddUint64x2", "interleave-odd", 64, 2, 2, 2, true, true, func(x, y, got []uint64) {
		shuffleInterleaveOddUint64x2(archsimd.LoadUint64x2(x), archsimd.LoadUint64x2(y)).Store(got)
	})
	check("broadcast1To2Float64x2", "broadcast-low", 64, 2, 0, 2, true, true, func(x, y, got []float64) {
		shufflebroadcast1To2Float64x2(archsimd.LoadFloat64x2(x)).Store(got)
	})
	check("broadcast1To2Int64x2", "broadcast-low", 64, 2, 0, 2, true, true, func(x, y, got []int64) {
		shufflebroadcast1To2Int64x2(archsimd.LoadInt64x2(x)).Store(got)
	})
	check("broadcast1To2Uint64x2", "broadcast-low", 64, 2, 0, 2, true, true, func(x, y, got []uint64) {
		shufflebroadcast1To2Uint64x2(archsimd.LoadUint64x2(x)).Store(got)
	})
	check("broadcast1To4Float32x4", "broadcast-low", 32, 4, 0, 4, true, true, func(x, y, got []float32) {
		shufflebroadcast1To4Float32x4(archsimd.LoadFloat32x4(x)).Store(got)
	})
	check("broadcast1To4Int32x4", "broadcast-low", 32, 4, 0, 4, true, true, func(x, y, got []int32) {
		shufflebroadcast1To4Int32x4(archsimd.LoadInt32x4(x)).Store(got)
	})
	check("broadcast1To4Uint32x4", "broadcast-low", 32, 4, 0, 4, true, true, func(x, y, got []uint32) {
		shufflebroadcast1To4Uint32x4(archsimd.LoadUint32x4(x)).Store(got)
	})
	check("broadcast1To8Int16x8", "broadcast-low", 16, 8, 0, 8, true, true, func(x, y, got []int16) {
		shufflebroadcast1To8Int16x8(archsimd.LoadInt16x8(x)).Store(got)
	})
	check("broadcast1To8Uint16x8", "broadcast-low", 16, 8, 0, 8, true, true, func(x, y, got []uint16) {
		shufflebroadcast1To8Uint16x8(archsimd.LoadUint16x8(x)).Store(got)
	})
	check("broadcast1To16Int8x16", "broadcast-low", 8, 16, 0, 16, true, true, func(x, y, got []int8) {
		shufflebroadcast1To16Int8x16(archsimd.LoadInt8x16(x)).Store(got)
	})
	check("broadcast1To16Uint8x16", "broadcast-low", 8, 16, 0, 16, true, true, func(x, y, got []uint8) {
		shufflebroadcast1To16Uint8x16(archsimd.LoadUint8x16(x)).Store(got)
	})
}
