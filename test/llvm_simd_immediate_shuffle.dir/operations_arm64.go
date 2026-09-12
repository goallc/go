// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func immediateConcatShiftBytesRightUint8x16(x, y archsimd.Uint8x16, c uint16) archsimd.Uint8x16 {
	return x.ConcatShiftBytesRight(y, uint64(c))
}
func checkArch() {
	checkImmediate("ConcatShiftBytesRightUint8x16", "ConcatShiftBytesRight", 8, 16, 16, true, true, func(x, y, got []uint8, c uint16) {
		immediateConcatShiftBytesRightUint8x16(archsimd.LoadUint8x16(x), archsimd.LoadUint8x16(y), c).Store(got)
	})
}
