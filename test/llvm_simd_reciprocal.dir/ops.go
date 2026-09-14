// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

// Slice arguments keep feature-disabled entry points on the scalar ABI.

//go:noinline
func recip32x4(x, reciprocal, rsqrt []float32) {
	if !archsimd.X86.AVX() {
		return
	}
	v := archsimd.LoadFloat32x4(x)
	v.Reciprocal().Store(reciprocal)
	v.ReciprocalSqrt().Store(rsqrt)
}

//go:noinline
func recip32x8(x, reciprocal, rsqrt []float32) {
	if !archsimd.X86.AVX() {
		return
	}
	v := archsimd.LoadFloat32x8(x)
	v.Reciprocal().Store(reciprocal)
	v.ReciprocalSqrt().Store(rsqrt)
}

//go:noinline
func recip32x16(x, reciprocal, rsqrt []float32) {
	if !archsimd.X86.AVX512() {
		return
	}
	v := archsimd.LoadFloat32x16(x)
	v.Reciprocal().Store(reciprocal)
	v.ReciprocalSqrt().Store(rsqrt)
}

//go:noinline
func recip64x2(x, reciprocal, rsqrt []float64) {
	if !archsimd.X86.AVX512() {
		return
	}
	v := archsimd.LoadFloat64x2(x)
	v.Reciprocal().Store(reciprocal)
	v.ReciprocalSqrt().Store(rsqrt)
}

//go:noinline
func recip64x4(x, reciprocal, rsqrt []float64) {
	if !archsimd.X86.AVX512() {
		return
	}
	v := archsimd.LoadFloat64x4(x)
	v.Reciprocal().Store(reciprocal)
	v.ReciprocalSqrt().Store(rsqrt)
}

//go:noinline
func recip64x8(x, reciprocal, rsqrt []float64) {
	if !archsimd.X86.AVX512() {
		return
	}
	v := archsimd.LoadFloat64x8(x)
	v.Reciprocal().Store(reciprocal)
	v.ReciprocalSqrt().Store(rsqrt)
}
