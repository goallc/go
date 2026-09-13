// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func widenInt8x16(x, y []int8, out []int16) {
	z := archsimd.LoadInt8x16(x).MulWidenLo(archsimd.LoadInt8x16(y))
	z.Store(out)
}

//go:noinline
func widenInt16x8(x, y []int16, out []int32) {
	z := archsimd.LoadInt16x8(x).MulWidenLo(archsimd.LoadInt16x8(y))
	z.Store(out)
}

//go:noinline
func widenInt32x4(x, y []int32, out []int64) {
	z := archsimd.LoadInt32x4(x).MulWidenLo(archsimd.LoadInt32x4(y))
	z.Store(out)
}

//go:noinline
func widenUint8x16(x, y []uint8, out []uint16) {
	z := archsimd.LoadUint8x16(x).MulWidenLo(archsimd.LoadUint8x16(y))
	z.Store(out)
}

//go:noinline
func widenUint16x8(x, y []uint16, out []uint32) {
	z := archsimd.LoadUint16x8(x).MulWidenLo(archsimd.LoadUint16x8(y))
	z.Store(out)
}

//go:noinline
func widenUint32x4(x, y []uint32, out []uint64) {
	z := archsimd.LoadUint32x4(x).MulWidenLo(archsimd.LoadUint32x4(y))
	z.Store(out)
}

func checkArch() {
	check("Int8x16", 16, false, true, widenInt8x16)
	check("Int16x8", 8, false, true, widenInt16x8)
	check("Int32x4", 4, false, true, widenInt32x4)
	check("Uint8x16", 16, false, true, widenUint8x16)
	check("Uint16x8", 8, false, true, widenUint16x8)
	check("Uint32x4", 4, false, true, widenUint32x4)
}
