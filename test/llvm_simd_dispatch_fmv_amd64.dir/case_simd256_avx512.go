// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// The @simd256 AVX2 floor combines with the AVX-512 FMV feature path.
func init() {
	registerDispatchFMVCase(dispatchFMVCase{
		order: 8, name: "simd256-avx512", godebug: "simd=+256",
		width: 256, emulated: false, usedAVX2: true, usedAVX512: true,
		support: supportAVX512,
	})
}
