// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func funnelInt16x8(x, y []int16, counts []uint16, count uint64, out []int16) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y)
		c := archsimd.LoadUint16x8(counts)
		a.ShiftAllLeftConcatMod16(b, count).Store(out[0:8])
		a.ShiftAllRightConcatMod16(b, count).Store(out[8:16])
		a.ShiftLeftConcatMod16(b, c).Store(out[16:24])
		a.ShiftRightConcatMod16(b, c).Store(out[24:32])
	}
}

//go:noinline
func funnelInt16x16(x, y []int16, counts []uint16, count uint64, out []int16) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadInt16x16(x), archsimd.LoadInt16x16(y)
		c := archsimd.LoadUint16x16(counts)
		a.ShiftAllLeftConcatMod16(b, count).Store(out[0:16])
		a.ShiftAllRightConcatMod16(b, count).Store(out[16:32])
		a.ShiftLeftConcatMod16(b, c).Store(out[32:48])
		a.ShiftRightConcatMod16(b, c).Store(out[48:64])
	}
}

//go:noinline
func funnelInt16x32(x, y []int16, counts []uint16, count uint64, out []int16) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadInt16x32(x), archsimd.LoadInt16x32(y)
		c := archsimd.LoadUint16x32(counts)
		a.ShiftAllLeftConcatMod16(b, count).Store(out[0:32])
		a.ShiftAllRightConcatMod16(b, count).Store(out[32:64])
		a.ShiftLeftConcatMod16(b, c).Store(out[64:96])
		a.ShiftRightConcatMod16(b, c).Store(out[96:128])
	}
}

//go:noinline
func funnelUint16x8(x, y []uint16, counts []uint16, count uint64, out []uint16) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadUint16x8(x), archsimd.LoadUint16x8(y)
		c := archsimd.LoadUint16x8(counts)
		a.ShiftAllLeftConcatMod16(b, count).Store(out[0:8])
		a.ShiftAllRightConcatMod16(b, count).Store(out[8:16])
		a.ShiftLeftConcatMod16(b, c).Store(out[16:24])
		a.ShiftRightConcatMod16(b, c).Store(out[24:32])
	}
}

//go:noinline
func funnelUint16x16(x, y []uint16, counts []uint16, count uint64, out []uint16) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadUint16x16(x), archsimd.LoadUint16x16(y)
		c := archsimd.LoadUint16x16(counts)
		a.ShiftAllLeftConcatMod16(b, count).Store(out[0:16])
		a.ShiftAllRightConcatMod16(b, count).Store(out[16:32])
		a.ShiftLeftConcatMod16(b, c).Store(out[32:48])
		a.ShiftRightConcatMod16(b, c).Store(out[48:64])
	}
}

//go:noinline
func funnelUint16x32(x, y []uint16, counts []uint16, count uint64, out []uint16) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadUint16x32(x), archsimd.LoadUint16x32(y)
		c := archsimd.LoadUint16x32(counts)
		a.ShiftAllLeftConcatMod16(b, count).Store(out[0:32])
		a.ShiftAllRightConcatMod16(b, count).Store(out[32:64])
		a.ShiftLeftConcatMod16(b, c).Store(out[64:96])
		a.ShiftRightConcatMod16(b, c).Store(out[96:128])
	}
}

//go:noinline
func funnelInt32x4(x, y []int32, counts []uint32, count uint64, out []int32) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y)
		c := archsimd.LoadUint32x4(counts)
		a.ShiftAllLeftConcatMod32(b, count).Store(out[0:4])
		a.ShiftAllRightConcatMod32(b, count).Store(out[4:8])
		a.ShiftLeftConcatMod32(b, c).Store(out[8:12])
		a.ShiftRightConcatMod32(b, c).Store(out[12:16])
	}
	if archsimd.X86.AVX512() {
		a := archsimd.LoadInt32x4(x)
		c := archsimd.LoadUint32x4(counts).AsInt32x4()
		a.RotateLeft(c).Store(out[16:20])
		a.RotateRight(c).Store(out[20:24])
	}
}

//go:noinline
func funnelInt32x8(x, y []int32, counts []uint32, count uint64, out []int32) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadInt32x8(x), archsimd.LoadInt32x8(y)
		c := archsimd.LoadUint32x8(counts)
		a.ShiftAllLeftConcatMod32(b, count).Store(out[0:8])
		a.ShiftAllRightConcatMod32(b, count).Store(out[8:16])
		a.ShiftLeftConcatMod32(b, c).Store(out[16:24])
		a.ShiftRightConcatMod32(b, c).Store(out[24:32])
	}
	if archsimd.X86.AVX512() {
		a := archsimd.LoadInt32x8(x)
		c := archsimd.LoadUint32x8(counts).AsInt32x8()
		a.RotateLeft(c).Store(out[32:40])
		a.RotateRight(c).Store(out[40:48])
	}
}

//go:noinline
func funnelInt32x16(x, y []int32, counts []uint32, count uint64, out []int32) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadInt32x16(x), archsimd.LoadInt32x16(y)
		c := archsimd.LoadUint32x16(counts)
		a.ShiftAllLeftConcatMod32(b, count).Store(out[0:16])
		a.ShiftAllRightConcatMod32(b, count).Store(out[16:32])
		a.ShiftLeftConcatMod32(b, c).Store(out[32:48])
		a.ShiftRightConcatMod32(b, c).Store(out[48:64])
	}
	if archsimd.X86.AVX512() {
		a := archsimd.LoadInt32x16(x)
		c := archsimd.LoadUint32x16(counts).AsInt32x16()
		a.RotateLeft(c).Store(out[64:80])
		a.RotateRight(c).Store(out[80:96])
	}
}

//go:noinline
func funnelUint32x4(x, y []uint32, counts []uint32, count uint64, out []uint32) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadUint32x4(x), archsimd.LoadUint32x4(y)
		c := archsimd.LoadUint32x4(counts)
		a.ShiftAllLeftConcatMod32(b, count).Store(out[0:4])
		a.ShiftAllRightConcatMod32(b, count).Store(out[4:8])
		a.ShiftLeftConcatMod32(b, c).Store(out[8:12])
		a.ShiftRightConcatMod32(b, c).Store(out[12:16])
	}
	if archsimd.X86.AVX512() {
		a := archsimd.LoadUint32x4(x)
		c := archsimd.LoadUint32x4(counts)
		a.RotateLeft(c).Store(out[16:20])
		a.RotateRight(c).Store(out[20:24])
	}
}

//go:noinline
func funnelUint32x8(x, y []uint32, counts []uint32, count uint64, out []uint32) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadUint32x8(x), archsimd.LoadUint32x8(y)
		c := archsimd.LoadUint32x8(counts)
		a.ShiftAllLeftConcatMod32(b, count).Store(out[0:8])
		a.ShiftAllRightConcatMod32(b, count).Store(out[8:16])
		a.ShiftLeftConcatMod32(b, c).Store(out[16:24])
		a.ShiftRightConcatMod32(b, c).Store(out[24:32])
	}
	if archsimd.X86.AVX512() {
		a := archsimd.LoadUint32x8(x)
		c := archsimd.LoadUint32x8(counts)
		a.RotateLeft(c).Store(out[32:40])
		a.RotateRight(c).Store(out[40:48])
	}
}

//go:noinline
func funnelUint32x16(x, y []uint32, counts []uint32, count uint64, out []uint32) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadUint32x16(x), archsimd.LoadUint32x16(y)
		c := archsimd.LoadUint32x16(counts)
		a.ShiftAllLeftConcatMod32(b, count).Store(out[0:16])
		a.ShiftAllRightConcatMod32(b, count).Store(out[16:32])
		a.ShiftLeftConcatMod32(b, c).Store(out[32:48])
		a.ShiftRightConcatMod32(b, c).Store(out[48:64])
	}
	if archsimd.X86.AVX512() {
		a := archsimd.LoadUint32x16(x)
		c := archsimd.LoadUint32x16(counts)
		a.RotateLeft(c).Store(out[64:80])
		a.RotateRight(c).Store(out[80:96])
	}
}

//go:noinline
func funnelInt64x2(x, y []int64, counts []uint64, count uint64, out []int64) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadInt64x2(x), archsimd.LoadInt64x2(y)
		c := archsimd.LoadUint64x2(counts)
		a.ShiftAllLeftConcatMod64(b, count).Store(out[0:2])
		a.ShiftAllRightConcatMod64(b, count).Store(out[2:4])
		a.ShiftLeftConcatMod64(b, c).Store(out[4:6])
		a.ShiftRightConcatMod64(b, c).Store(out[6:8])
	}
	if archsimd.X86.AVX512() {
		a := archsimd.LoadInt64x2(x)
		c := archsimd.LoadUint64x2(counts).AsInt64x2()
		a.RotateLeft(c).Store(out[8:10])
		a.RotateRight(c).Store(out[10:12])
	}
}

//go:noinline
func funnelInt64x4(x, y []int64, counts []uint64, count uint64, out []int64) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadInt64x4(x), archsimd.LoadInt64x4(y)
		c := archsimd.LoadUint64x4(counts)
		a.ShiftAllLeftConcatMod64(b, count).Store(out[0:4])
		a.ShiftAllRightConcatMod64(b, count).Store(out[4:8])
		a.ShiftLeftConcatMod64(b, c).Store(out[8:12])
		a.ShiftRightConcatMod64(b, c).Store(out[12:16])
	}
	if archsimd.X86.AVX512() {
		a := archsimd.LoadInt64x4(x)
		c := archsimd.LoadUint64x4(counts).AsInt64x4()
		a.RotateLeft(c).Store(out[16:20])
		a.RotateRight(c).Store(out[20:24])
	}
}

//go:noinline
func funnelInt64x8(x, y []int64, counts []uint64, count uint64, out []int64) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadInt64x8(x), archsimd.LoadInt64x8(y)
		c := archsimd.LoadUint64x8(counts)
		a.ShiftAllLeftConcatMod64(b, count).Store(out[0:8])
		a.ShiftAllRightConcatMod64(b, count).Store(out[8:16])
		a.ShiftLeftConcatMod64(b, c).Store(out[16:24])
		a.ShiftRightConcatMod64(b, c).Store(out[24:32])
	}
	if archsimd.X86.AVX512() {
		a := archsimd.LoadInt64x8(x)
		c := archsimd.LoadUint64x8(counts).AsInt64x8()
		a.RotateLeft(c).Store(out[32:40])
		a.RotateRight(c).Store(out[40:48])
	}
}

//go:noinline
func funnelUint64x2(x, y []uint64, counts []uint64, count uint64, out []uint64) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadUint64x2(x), archsimd.LoadUint64x2(y)
		c := archsimd.LoadUint64x2(counts)
		a.ShiftAllLeftConcatMod64(b, count).Store(out[0:2])
		a.ShiftAllRightConcatMod64(b, count).Store(out[2:4])
		a.ShiftLeftConcatMod64(b, c).Store(out[4:6])
		a.ShiftRightConcatMod64(b, c).Store(out[6:8])
	}
	if archsimd.X86.AVX512() {
		a := archsimd.LoadUint64x2(x)
		c := archsimd.LoadUint64x2(counts)
		a.RotateLeft(c).Store(out[8:10])
		a.RotateRight(c).Store(out[10:12])
	}
}

//go:noinline
func funnelUint64x4(x, y []uint64, counts []uint64, count uint64, out []uint64) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadUint64x4(x), archsimd.LoadUint64x4(y)
		c := archsimd.LoadUint64x4(counts)
		a.ShiftAllLeftConcatMod64(b, count).Store(out[0:4])
		a.ShiftAllRightConcatMod64(b, count).Store(out[4:8])
		a.ShiftLeftConcatMod64(b, c).Store(out[8:12])
		a.ShiftRightConcatMod64(b, c).Store(out[12:16])
	}
	if archsimd.X86.AVX512() {
		a := archsimd.LoadUint64x4(x)
		c := archsimd.LoadUint64x4(counts)
		a.RotateLeft(c).Store(out[16:20])
		a.RotateRight(c).Store(out[20:24])
	}
}

//go:noinline
func funnelUint64x8(x, y []uint64, counts []uint64, count uint64, out []uint64) {
	if archsimd.X86.AVX512VBMI2() {
		a, b := archsimd.LoadUint64x8(x), archsimd.LoadUint64x8(y)
		c := archsimd.LoadUint64x8(counts)
		a.ShiftAllLeftConcatMod64(b, count).Store(out[0:8])
		a.ShiftAllRightConcatMod64(b, count).Store(out[8:16])
		a.ShiftLeftConcatMod64(b, c).Store(out[16:24])
		a.ShiftRightConcatMod64(b, c).Store(out[24:32])
	}
	if archsimd.X86.AVX512() {
		a := archsimd.LoadUint64x8(x)
		c := archsimd.LoadUint64x8(counts)
		a.RotateLeft(c).Store(out[32:40])
		a.RotateRight(c).Store(out[40:48])
	}
}

func main() {
	check("Int16x8", 16, 8, funnelInt16x8)
	check("Int16x16", 16, 16, funnelInt16x16)
	check("Int16x32", 16, 32, funnelInt16x32)
	check("Uint16x8", 16, 8, funnelUint16x8)
	check("Uint16x16", 16, 16, funnelUint16x16)
	check("Uint16x32", 16, 32, funnelUint16x32)
	check("Int32x4", 32, 4, funnelInt32x4)
	check("Int32x8", 32, 8, funnelInt32x8)
	check("Int32x16", 32, 16, funnelInt32x16)
	check("Uint32x4", 32, 4, funnelUint32x4)
	check("Uint32x8", 32, 8, funnelUint32x8)
	check("Uint32x16", 32, 16, funnelUint32x16)
	check("Int64x2", 64, 2, funnelInt64x2)
	check("Int64x4", 64, 4, funnelInt64x4)
	check("Int64x8", 64, 8, funnelInt64x8)
	check("Uint64x2", 64, 2, funnelUint64x2)
	check("Uint64x4", 64, 4, funnelUint64x4)
	check("Uint64x8", 64, 8, funnelUint64x8)
	if trace {
		fmtTrace()
	}
}
