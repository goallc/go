// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func fmaFloat32x4(x, y, z, out []float32) {
	if !archsimd.X86.FMA() {
		return
	}
	a, b, c := archsimd.LoadFloat32x4(x), archsimd.LoadFloat32x4(y), archsimd.LoadFloat32x4(z)
	a.MulAdd(b, c).Store(out[:4])
	a.MulAddEvenSubOdd(b, c).Store(out[4:8])
	a.MulAddOddSubEven(b, c).Store(out[8:])
}

//go:noinline
func fmaFloat32x8(x, y, z, out []float32) {
	if !archsimd.X86.FMA() {
		return
	}
	a, b, c := archsimd.LoadFloat32x8(x), archsimd.LoadFloat32x8(y), archsimd.LoadFloat32x8(z)
	a.MulAdd(b, c).Store(out[:8])
	a.MulAddEvenSubOdd(b, c).Store(out[8:16])
	a.MulAddOddSubEven(b, c).Store(out[16:])
}

//go:noinline
func fmaFloat32x16(x, y, z, out []float32) {
	if !archsimd.X86.AVX512() {
		return
	}
	a, b, c := archsimd.LoadFloat32x16(x), archsimd.LoadFloat32x16(y), archsimd.LoadFloat32x16(z)
	a.MulAdd(b, c).Store(out[:16])
	a.MulAddEvenSubOdd(b, c).Store(out[16:32])
	a.MulAddOddSubEven(b, c).Store(out[32:])
}

//go:noinline
func fmaFloat64x2(x, y, z, out []float64) {
	if !archsimd.X86.FMA() {
		return
	}
	a, b, c := archsimd.LoadFloat64x2(x), archsimd.LoadFloat64x2(y), archsimd.LoadFloat64x2(z)
	a.MulAdd(b, c).Store(out[:2])
	a.MulAddEvenSubOdd(b, c).Store(out[2:4])
	a.MulAddOddSubEven(b, c).Store(out[4:])
}

//go:noinline
func fmaFloat64x4(x, y, z, out []float64) {
	if !archsimd.X86.FMA() {
		return
	}
	a, b, c := archsimd.LoadFloat64x4(x), archsimd.LoadFloat64x4(y), archsimd.LoadFloat64x4(z)
	a.MulAdd(b, c).Store(out[:4])
	a.MulAddEvenSubOdd(b, c).Store(out[4:8])
	a.MulAddOddSubEven(b, c).Store(out[8:])
}

//go:noinline
func fmaFloat64x8(x, y, z, out []float64) {
	if !archsimd.X86.AVX512() {
		return
	}
	a, b, c := archsimd.LoadFloat64x8(x), archsimd.LoadFloat64x8(y), archsimd.LoadFloat64x8(z)
	a.MulAdd(b, c).Store(out[:8])
	a.MulAddEvenSubOdd(b, c).Store(out[8:16])
	a.MulAddOddSubEven(b, c).Store(out[16:])
}

func checkArch() {
	check("Float32x4", 4, 3, archsimd.X86.FMA(), fmaFloat32x4)
	check("Float32x8", 8, 3, archsimd.X86.FMA(), fmaFloat32x8)
	check("Float32x16", 16, 3, archsimd.X86.AVX512(), fmaFloat32x16)
	check("Float64x2", 2, 3, archsimd.X86.FMA(), fmaFloat64x2)
	check("Float64x4", 4, 3, archsimd.X86.FMA(), fmaFloat64x4)
	check("Float64x8", 8, 3, archsimd.X86.AVX512(), fmaFloat64x8)
}
