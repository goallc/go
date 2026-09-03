// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"simd"
	"simd/archsimd"
)

// Keep a real wide-vector call boundary in each guarded path. The identity
// callees need only their ABI floor; the AVX2 and AVX-512 operations remain in
// dispatchFMVOperation, where their matching fallback is visible to FMV.

//go:noinline
func dispatchFMVIdentity256(x archsimd.Int8x32) archsimd.Int8x32 {
	return x
}

//go:noinline
func dispatchFMVIdentity512(x archsimd.Int8x64) archsimd.Int8x64 {
	return x
}

// dispatchFMVOperation has a scalar external ABI, so Midway emits the public
// width dispatcher and @simd0/@simd128/@simd256/@simd512 variants.
//
// Within each width variant:
//
//	@simd0:   no floor; FMV handles AVX2 and AVX-512
//	@simd128: AVX floor; FMV still handles AVX2 and AVX-512
//	@simd256: AVX2 floor; only AVX-512 still needs FMV
//	@simd512: AVX-512 floor; neither guard needs FMV
//
//go:noinline
func dispatchFMVOperation(
	portableDst *[64]int8,
	wide256Dst *[32]int8,
	wide512Dst *[64]int8,
	x, y *[64]int8,
	x256, y256 *[32]int8,
) (lanes int, usedAVX2, usedAVX512 bool) {
	portable := simd.LoadInt8s(x[:]).Add(simd.LoadInt8s(y[:]))
	portable.Store(portableDst[:])

	if archsimd.X86.AVX2() {
		value := archsimd.LoadInt8x32Array(x256).Add(archsimd.LoadInt8x32Array(y256))
		dispatchFMVIdentity256(value).StoreArray(wide256Dst)
		usedAVX2 = true
	} else {
		for i := range wide256Dst {
			wide256Dst[i] = x256[i] + y256[i]
		}
	}

	if archsimd.X86.AVX512() {
		value := archsimd.LoadInt8x64Array(x).Add(archsimd.LoadInt8x64Array(y))
		dispatchFMVIdentity512(value).StoreArray(wide512Dst)
		usedAVX512 = true
	} else {
		for i := range wide512Dst {
			wide512Dst[i] = x[i] + y[i]
		}
	}

	return portable.Len(), usedAVX2, usedAVX512
}
