// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// The @simd128 AVX floor covers neither guarded profile, so both fall back.
func init() {
	registerDispatchFMVCase(dispatchFMVCase{
		order: 4, name: "simd128-fallbacks", godebug: "simd=+128,cpu.avx2=off,cpu.avx512f=off",
		width: 128, emulated: false, usedAVX2: false, usedAVX512: false,
		support: supportAVX,
	})
}
