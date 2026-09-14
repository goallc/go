// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func residue32x4(x []float32, out [4][]float32, prec uint8) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat32x4(x)
	a.RoundScaledResidue(prec).Store(out[0])
	a.FloorScaledResidue(prec).Store(out[1])
	a.CeilScaledResidue(prec).Store(out[2])
	a.TruncScaledResidue(prec).Store(out[3])
}

//go:noinline
func residue32x8(x []float32, out [4][]float32, prec uint8) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat32x8(x)
	a.RoundScaledResidue(prec).Store(out[0])
	a.FloorScaledResidue(prec).Store(out[1])
	a.CeilScaledResidue(prec).Store(out[2])
	a.TruncScaledResidue(prec).Store(out[3])
}

//go:noinline
func residue32x16(x []float32, out [4][]float32, prec uint8) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat32x16(x)
	a.RoundScaledResidue(prec).Store(out[0])
	a.FloorScaledResidue(prec).Store(out[1])
	a.CeilScaledResidue(prec).Store(out[2])
	a.TruncScaledResidue(prec).Store(out[3])
}

//go:noinline
func residue64x2(x []float64, out [4][]float64, prec uint8) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat64x2(x)
	a.RoundScaledResidue(prec).Store(out[0])
	a.FloorScaledResidue(prec).Store(out[1])
	a.CeilScaledResidue(prec).Store(out[2])
	a.TruncScaledResidue(prec).Store(out[3])
}

//go:noinline
func residue64x4(x []float64, out [4][]float64, prec uint8) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat64x4(x)
	a.RoundScaledResidue(prec).Store(out[0])
	a.FloorScaledResidue(prec).Store(out[1])
	a.CeilScaledResidue(prec).Store(out[2])
	a.TruncScaledResidue(prec).Store(out[3])
}

//go:noinline
func residue64x8(x []float64, out [4][]float64, prec uint8) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat64x8(x)
	a.RoundScaledResidue(prec).Store(out[0])
	a.FloorScaledResidue(prec).Store(out[1])
	a.CeilScaledResidue(prec).Store(out[2])
	a.TruncScaledResidue(prec).Store(out[3])
}

//go:noinline
func selectBytes(x, y, out []int8) {
	if !archsimd.X86.AVX2() {
		return
	}
	a, b := archsimd.LoadInt8x32(x), archsimd.LoadInt8x32(y)
	a.IfElse(a.Greater(b), b).Store(out)
}
