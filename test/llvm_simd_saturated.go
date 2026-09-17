//go:build amd64 || arm64

// run -goexperiment simd

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"runtime"
	"simd/archsimd"
)

//go:noinline
func saturatedSigned(x, y archsimd.Int8x16) (archsimd.Int8x16, archsimd.Int8x16) {
	return x.AddSaturated(y), x.SubSaturated(y)
}

//go:noinline
func saturatedUnsigned(x, y archsimd.Uint16x8) (archsimd.Uint16x8, archsimd.Uint16x8) {
	return x.AddSaturated(y), x.SubSaturated(y)
}

func main() {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}

	signedX := [16]int8{127, 126, -128, -127, 100, -100, 1, -1, 0, 50, -50, 64, -64, 12, -12, 42}
	signedY := [16]int8{1, 2, -1, -2, 100, -100, -2, 2, 0, 77, -77, 64, -65, -12, 12, -100}
	wantSignedAdd := [16]int8{127, 127, -128, -128, 127, -128, -1, 1, 0, 127, -127, 127, -128, 0, 0, -58}
	wantSignedSub := [16]int8{126, 124, -127, -125, 0, 0, 3, -3, 0, -27, 27, 0, 1, 24, -24, 127}
	gotSignedAdd, gotSignedSub := saturatedSigned(
		archsimd.LoadInt8x16Array(&signedX),
		archsimd.LoadInt8x16Array(&signedY),
	)
	var signedAdd, signedSub [16]int8
	gotSignedAdd.StoreArray(&signedAdd)
	gotSignedSub.StoreArray(&signedSub)
	for i := range signedAdd {
		if signedAdd[i] != wantSignedAdd[i] || signedSub[i] != wantSignedSub[i] {
			panic(i)
		}
	}

	unsignedX := [8]uint16{65535, 65534, 0, 1, 40000, 100, 500, 42}
	unsignedY := [8]uint16{1, 2, 1, 2, 40000, 200, 499, 100}
	wantUnsignedAdd := [8]uint16{65535, 65535, 1, 3, 65535, 300, 999, 142}
	wantUnsignedSub := [8]uint16{65534, 65532, 0, 0, 0, 0, 1, 0}
	gotUnsignedAdd, gotUnsignedSub := saturatedUnsigned(
		archsimd.LoadUint16x8Array(&unsignedX),
		archsimd.LoadUint16x8Array(&unsignedY),
	)
	var unsignedAdd, unsignedSub [8]uint16
	gotUnsignedAdd.StoreArray(&unsignedAdd)
	gotUnsignedSub.StoreArray(&unsignedSub)
	for i := range unsignedAdd {
		if unsignedAdd[i] != wantUnsignedAdd[i] || unsignedSub[i] != wantUnsignedSub[i] {
			panic(16 + i)
		}
	}
}
