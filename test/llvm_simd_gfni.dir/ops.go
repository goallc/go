// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

// Keep the entry ABI callable on CPUs without AVX512GFNI.
//
//go:noinline
func gf128(x, y [64]byte, matrix [8]uint64, b uint8) (out [3][64]byte) {
	if !archsimd.X86.AVX512GFNI() {
		return
	}
	v := archsimd.LoadUint8x16(x[:16])
	m := archsimd.LoadUint64x2(matrix[:2])
	v.GaloisFieldMul(archsimd.LoadUint8x16(y[:16])).Store(out[0][:16])
	v.GaloisFieldAffineTransform(m, b).Store(out[1][:16])
	v.GaloisFieldAffineTransformInverse(m, b).Store(out[2][:16])
	return
}

//go:noinline
func gf256(x, y [64]byte, matrix [8]uint64, b uint8) (out [3][64]byte) {
	if !archsimd.X86.AVX512GFNI() {
		return
	}
	v := archsimd.LoadUint8x32(x[:32])
	m := archsimd.LoadUint64x4(matrix[:4])
	v.GaloisFieldMul(archsimd.LoadUint8x32(y[:32])).Store(out[0][:32])
	v.GaloisFieldAffineTransform(m, b).Store(out[1][:32])
	v.GaloisFieldAffineTransformInverse(m, b).Store(out[2][:32])
	return
}

//go:noinline
func gf512(x, y [64]byte, matrix [8]uint64, b uint8) (out [3][64]byte) {
	if !archsimd.X86.AVX512GFNI() {
		return
	}
	v := archsimd.LoadUint8x64(x[:])
	m := archsimd.LoadUint64x8(matrix[:])
	v.GaloisFieldMul(archsimd.LoadUint8x64(y[:])).Store(out[0][:])
	v.GaloisFieldAffineTransform(m, b).Store(out[1][:])
	v.GaloisFieldAffineTransformInverse(m, b).Store(out[2][:])
	return
}
