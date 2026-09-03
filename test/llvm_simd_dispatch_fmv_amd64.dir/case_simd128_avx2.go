// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// The @simd128 AVX floor combines with the AVX2 FMV feature implementation.
func init() {
	registerDispatchFMVCase(dispatchFMVCase{
		order: 5, name: "simd128-avx2", godebug: "simd=+128,cpu.avx512f=off",
		width: 128, emulated: false, usedAVX2: true, usedAVX512: false,
		support: supportAVX2,
	})
}
