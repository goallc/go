// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

// Slice arguments keep the fallback entry ABI independent of AVX512.

//go:noinline
func scale32x4(x, y, out []float32) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat32x4(x)
	b := archsimd.LoadFloat32x4(y)
	a.Scale(b).Store(out)
}

//go:noinline
func scale32x8(x, y, out []float32) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat32x8(x)
	b := archsimd.LoadFloat32x8(y)
	a.Scale(b).Store(out)
}

//go:noinline
func scale32x16(x, y, out []float32) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat32x16(x)
	b := archsimd.LoadFloat32x16(y)
	a.Scale(b).Store(out)
}

//go:noinline
func scale64x2(x, y, out []float64) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat64x2(x)
	b := archsimd.LoadFloat64x2(y)
	a.Scale(b).Store(out)
}

//go:noinline
func scale64x4(x, y, out []float64) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat64x4(x)
	b := archsimd.LoadFloat64x4(y)
	a.Scale(b).Store(out)
}

//go:noinline
func scale64x8(x, y, out []float64) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat64x8(x)
	b := archsimd.LoadFloat64x8(y)
	a.Scale(b).Store(out)
}
