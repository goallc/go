// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func dynamicLookupOrZeroInt8x16(x, y archsimd.Int8x16, indices archsimd.Int8x16) archsimd.Int8x16 {
	return x.LookupOrZero(indices)
}

//go:noinline
func dynamicLookupOrZeroUint8x16(x, y archsimd.Uint8x16, indices archsimd.Uint8x16) archsimd.Uint8x16 {
	return x.LookupOrZero(indices)
}

func checkArch() {
	checkDynamic[int8, int8]("LookupOrZeroInt8x16", "LookupOrZero", 8, 16, true, true, func(x, y []int8, indices []int8, got []int8) {
		z := dynamicLookupOrZeroInt8x16(archsimd.LoadInt8x16(x), archsimd.LoadInt8x16(y), archsimd.LoadInt8x16(indices))
		z.Store(got)
	})
	checkDynamic[uint8, uint8]("LookupOrZeroUint8x16", "LookupOrZero", 8, 16, true, true, func(x, y []uint8, indices []uint8, got []uint8) {
		z := dynamicLookupOrZeroUint8x16(archsimd.LoadUint8x16(x), archsimd.LoadUint8x16(y), archsimd.LoadUint8x16(indices))
		z.Store(got)
	})
}
