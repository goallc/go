// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// The @simd512 floor covers both profiles, so no nested FMV is required.
func init() {
	registerDispatchFMVCase(dispatchFMVCase{
		order: 9, name: "simd512-floor", godebug: "simd=+512",
		width: 512, emulated: false, usedAVX2: true, usedAVX512: true,
		support: supportAVX512,
	})
}
