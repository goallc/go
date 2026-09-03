// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// The @simd128 AVX floor combines with both FMV feature implementations.
func init() {
	registerDispatchFMVCase(dispatchFMVCase{
		order: 6, name: "simd128-all-features", godebug: "simd=+128",
		width: 128, emulated: false, usedAVX2: true, usedAVX512: true,
		support: supportAVX512,
	})
}
