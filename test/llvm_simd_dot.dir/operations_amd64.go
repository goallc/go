// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func dot128(x, y []int16, u []uint8, s []int8, d []int32, sat []int16, sad []uint64) {
	if !archsimd.X86.AVX() {
		return
	}
	archsimd.LoadInt16x8(x).DotProductPairs(archsimd.LoadInt16x8(y)).Store(d)
	a := archsimd.LoadUint8x16(u)
	a.DotProductPairsSaturated(archsimd.LoadInt8x16(s)).Store(sat)
	a.SumOf8AbsDiff(archsimd.LoadInt8x16(s).AsUint8x16()).Store(sad)
}

//go:noinline
func dot256(x, y []int16, u []uint8, s []int8, d []int32, sat []int16, sad []uint64) {
	if !archsimd.X86.AVX2() {
		return
	}
	archsimd.LoadInt16x16(x).DotProductPairs(archsimd.LoadInt16x16(y)).Store(d)
	a := archsimd.LoadUint8x32(u)
	a.DotProductPairsSaturated(archsimd.LoadInt8x32(s)).Store(sat)
	a.SumOf8AbsDiff(archsimd.LoadInt8x32(s).AsUint8x32()).Store(sad)
}

//go:noinline
func dot512(x, y []int16, u []uint8, s []int8, d []int32, sat []int16, sad []uint64) {
	if !archsimd.X86.AVX512() {
		return
	}
	archsimd.LoadInt16x32(x).DotProductPairs(archsimd.LoadInt16x32(y)).Store(d)
	a := archsimd.LoadUint8x64(u)
	a.DotProductPairsSaturated(archsimd.LoadInt8x64(s)).Store(sat)
	a.SumOf8AbsDiff(archsimd.LoadInt8x64(s).AsUint8x64()).Store(sad)
}

func main() {
	check(128, archsimd.X86.AVX(), dot128)
	check(256, archsimd.X86.AVX2(), dot256)
	check(512, archsimd.X86.AVX512(), dot512)
	if trace {
		fmtTrace()
	}
}
