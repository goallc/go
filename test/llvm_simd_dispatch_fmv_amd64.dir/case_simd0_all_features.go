// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// @simd0 selects the combined AVX2 plus AVX-512 FMV implementation.
func init() {
	registerDispatchFMVCase(dispatchFMVCase{
		order: 3, name: "simd0-all-features", godebug: "simd=0",
		width: 128, emulated: true, usedAVX2: true, usedAVX512: true,
		support: supportAVX512,
	})
}
