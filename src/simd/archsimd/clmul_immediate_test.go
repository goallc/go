// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && amd64

package archsimd_test

import (
	"simd/archsimd"
	"testing"
)

func TestCarrylessMultiplyImmediate(t *testing.T) {
	if !archsimd.X86.AVXPCLMULQDQ() {
		t.Skip("requires AVXPCLMULQDQ")
	}
	x := archsimd.LoadUint64x2([]uint64{1, 5})
	y := archsimd.LoadUint64x2([]uint64{3, 9})
	check := func(imm uint8, product archsimd.Uint64x2) {
		want := [4]uint64{3, 15, 9, 45}[(imm&1)|((imm>>3)&2)]
		var got [2]uint64
		product.Store(got[:])
		if got != [2]uint64{want, 0} {
			t.Fatalf("imm=%#x: got %v, want [%d 0]", imm, got, want)
		}
	}
	check(0x80, x.ExportTestCarrylessMultiply80(y))
	check(0xff, x.ExportTestCarrylessMultiplyFF(y))
	for imm := 0; imm < 256; imm++ {
		check(uint8(imm), x.ExportTestCarrylessMultiply(uint8(imm), y))
	}
}
