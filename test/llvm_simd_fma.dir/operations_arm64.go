// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func fmaFloat32x4(x, y, z, out []float32) {
	a, b, c := archsimd.LoadFloat32x4(x), archsimd.LoadFloat32x4(y), archsimd.LoadFloat32x4(z)
	a.MulAdd(b, c).Store(out[:4])
}

//go:noinline
func fmaFloat64x2(x, y, z, out []float64) {
	a, b, c := archsimd.LoadFloat64x2(x), archsimd.LoadFloat64x2(y), archsimd.LoadFloat64x2(z)
	a.MulAdd(b, c).Store(out[:2])
}

func checkArch() {
	check("Float32x4", 4, 1, true, fmaFloat32x4)
	check("Float64x2", 2, 1, true, fmaFloat64x2)
}
