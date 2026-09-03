// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// @simd0 has no floor, so both guarded operations use their FMV fallbacks.
func init() {
	registerDispatchFMVCase(dispatchFMVCase{
		order: 1, name: "simd0-all-off", godebug: "simd=0,cpu.all=off",
		width: 128, emulated: true, usedAVX2: false, usedAVX512: false,
		support: supportAlways,
	})
}
