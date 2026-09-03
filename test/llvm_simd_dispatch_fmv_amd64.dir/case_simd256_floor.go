// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// The @simd256 floor covers AVX2; only AVX-512 keeps an FMV fallback.
func init() {
	registerDispatchFMVCase(dispatchFMVCase{
		order: 7, name: "simd256-floor", godebug: "simd=+256,cpu.avx512f=off",
		width: 256, emulated: false, usedAVX2: true, usedAVX512: false,
		support: supportAVX2,
	})
}
