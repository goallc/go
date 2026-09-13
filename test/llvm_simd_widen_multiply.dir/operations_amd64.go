// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func widenInt32x4(x, y []int32, out []int64) {
	if !archsimd.X86.AVX() {
		for i := range out {
			out[i] = 0
		}
		return
	}
	z := archsimd.LoadInt32x4(x).MulWidenEven(archsimd.LoadInt32x4(y))
	z.Store(out)
}

//go:noinline
func widenInt32x8(x, y []int32, out []int64) {
	if !archsimd.X86.AVX2() {
		for i := range out {
			out[i] = 0
		}
		return
	}
	z := archsimd.LoadInt32x8(x).MulWidenEven(archsimd.LoadInt32x8(y))
	z.Store(out)
}

//go:noinline
func widenUint32x4(x, y []uint32, out []uint64) {
	if !archsimd.X86.AVX() {
		for i := range out {
			out[i] = 0
		}
		return
	}
	z := archsimd.LoadUint32x4(x).MulWidenEven(archsimd.LoadUint32x4(y))
	z.Store(out)
}

//go:noinline
func widenUint32x8(x, y []uint32, out []uint64) {
	if !archsimd.X86.AVX2() {
		for i := range out {
			out[i] = 0
		}
		return
	}
	z := archsimd.LoadUint32x8(x).MulWidenEven(archsimd.LoadUint32x8(y))
	z.Store(out)
}

func checkArch() {
	check("Int32x4", 4, true, archsimd.X86.AVX(), widenInt32x4)
	check("Int32x8", 8, true, archsimd.X86.AVX2(), widenInt32x8)
	check("Uint32x4", 4, true, archsimd.X86.AVX(), widenUint32x4)
	check("Uint32x8", 8, true, archsimd.X86.AVX2(), widenUint32x8)
}
