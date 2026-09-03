// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// @simd0 selects the AVX2 FMV path while AVX-512 remains on its fallback.
func init() {
	registerDispatchFMVCase(dispatchFMVCase{
		order: 2, name: "simd0-avx2", godebug: "simd=0,cpu.avx512f=off",
		width: 128, emulated: true, usedAVX2: true, usedAVX512: false,
		support: supportAVX2,
	})
}
