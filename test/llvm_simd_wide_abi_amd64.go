//go:build amd64

// run -goexperiment simd -llvm-package-only

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func wideABIIdentity256(x archsimd.Uint8x32) archsimd.Uint8x32 {
	return x
}

//go:noinline
func guardedWideABICall256(dst, src *[32]uint8) {
	if !archsimd.X86.AVX2() {
		copy(dst[:], src[:])
		return
	}
	wideABIIdentity256(archsimd.LoadUint8x32Array(src)).StoreArray(dst)
}

//go:noinline
func wideABIIdentity512(x archsimd.Uint8x64) archsimd.Uint8x64 {
	return x
}

//go:noinline
func guardedWideABICall512(dst, src *[64]uint8) {
	if !archsimd.X86.AVX512() {
		copy(dst[:], src[:])
		return
	}
	wideABIIdentity512(archsimd.LoadUint8x64Array(src)).StoreArray(dst)
}

func main() {
	var src256, got256 [32]uint8
	for i := range src256 {
		src256[i] = uint8(i*7 + 3)
	}
	guardedWideABICall256(&got256, &src256)
	if got256 != src256 {
		panic("incorrect 256-bit call result")
	}

	var src512, got512 [64]uint8
	for i := range src512 {
		src512[i] = uint8(i*5 + 1)
	}
	guardedWideABICall512(&got512, &src512)
	if got512 != src512 {
		panic("incorrect 512-bit call result")
	}
}
