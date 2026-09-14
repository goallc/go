// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

// Slice arguments keep the fallback entry ABI independent of AVX512.

//go:noinline
func round32x4(x []float32, out [4][]float32, prec uint8) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat32x4(x)
	a.RoundScaled(prec).Store(out[0])
	a.FloorScaled(prec).Store(out[1])
	a.CeilScaled(prec).Store(out[2])
	a.TruncScaled(prec).Store(out[3])
}

//go:noinline
func round32x8(x []float32, out [4][]float32, prec uint8) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat32x8(x)
	a.RoundScaled(prec).Store(out[0])
	a.FloorScaled(prec).Store(out[1])
	a.CeilScaled(prec).Store(out[2])
	a.TruncScaled(prec).Store(out[3])
}

//go:noinline
func round32x16(x []float32, out [4][]float32, prec uint8) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat32x16(x)
	a.RoundScaled(prec).Store(out[0])
	a.FloorScaled(prec).Store(out[1])
	a.CeilScaled(prec).Store(out[2])
	a.TruncScaled(prec).Store(out[3])
}

//go:noinline
func round64x2(x []float64, out [4][]float64, prec uint8) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat64x2(x)
	a.RoundScaled(prec).Store(out[0])
	a.FloorScaled(prec).Store(out[1])
	a.CeilScaled(prec).Store(out[2])
	a.TruncScaled(prec).Store(out[3])
}

//go:noinline
func round64x4(x []float64, out [4][]float64, prec uint8) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat64x4(x)
	a.RoundScaled(prec).Store(out[0])
	a.FloorScaled(prec).Store(out[1])
	a.CeilScaled(prec).Store(out[2])
	a.TruncScaled(prec).Store(out[3])
}

//go:noinline
func round64x8(x []float64, out [4][]float64, prec uint8) {
	if !archsimd.X86.AVX512() {
		return
	}
	a := archsimd.LoadFloat64x8(x)
	a.RoundScaled(prec).Store(out[0])
	a.FloorScaled(prec).Store(out[1])
	a.CeilScaled(prec).Store(out[2])
	a.TruncScaled(prec).Store(out[3])
}
