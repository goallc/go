//go:build amd64

// run -goexperiment simd -llvm-package-only

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func composeMulHighSigned(x, y archsimd.Int16x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX() {
		return x
	}
	return x.MulHigh(y)
}

//go:noinline
func composeMulHighUnsigned(x, y archsimd.Uint16x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX() {
		return x
	}
	return x.MulHigh(y)
}

//go:noinline
func composeMulSign(x, y archsimd.Int8x16) archsimd.Int8x16 {
	if !archsimd.X86.AVX() {
		return x
	}
	return x.MulSign(y)
}

//go:noinline
func composeAverageUnsigned256(x, y archsimd.Uint8x32) archsimd.Uint8x32 {
	if !archsimd.X86.AVX2() {
		return x
	}
	return x.Average(y)
}

//go:noinline
func composeMulHighSigned256(x, y archsimd.Int16x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return x
	}
	return x.MulHigh(y)
}

//go:noinline
func composeMulSign256(x, y archsimd.Int8x32) archsimd.Int8x32 {
	if !archsimd.X86.AVX2() {
		return x
	}
	return x.MulSign(y)
}

func main() {
	if !archsimd.X86.AVX() {
		return
	}

	sx := [8]int16{-32768, -32768, -30000, -1, 0, 1, 30000, 32767}
	sy := [8]int16{-32768, 32767, -30000, 32767, -1, 1, 30000, 32767}
	signed := composeMulHighSigned(archsimd.LoadInt16x8Array(&sx), archsimd.LoadInt16x8Array(&sy))
	var gotSigned [8]int16
	signed.StoreArray(&gotSigned)
	for i := range gotSigned {
		want := int16((int32(sx[i]) * int32(sy[i])) >> 16)
		if gotSigned[i] != want {
			panic(i)
		}
	}

	ux := [8]uint16{0, 1, 2, 255, 256, 32768, 65534, 65535}
	uy := [8]uint16{65535, 65535, 32768, 257, 256, 32768, 65535, 65535}
	unsigned := composeMulHighUnsigned(archsimd.LoadUint16x8Array(&ux), archsimd.LoadUint16x8Array(&uy))
	var gotUnsigned [8]uint16
	unsigned.StoreArray(&gotUnsigned)
	for i := range gotUnsigned {
		want := uint16((uint32(ux[i]) * uint32(uy[i])) >> 16)
		if gotUnsigned[i] != want {
			panic(8 + i)
		}
	}

	mx := [16]int8{-128, -127, -64, -1, 0, 1, 2, 3, 4, 5, 63, 64, 65, 126, 127, -42}
	my := [16]int8{-1, 0, 1, -128, -1, 0, 1, 127, -7, 9, 0, -1, 1, -2, 2, 0}
	signedBySign := composeMulSign(archsimd.LoadInt8x16Array(&mx), archsimd.LoadInt8x16Array(&my))
	var gotMulSign [16]int8
	signedBySign.StoreArray(&gotMulSign)
	for i := range gotMulSign {
		var want int8
		switch {
		case my[i] < 0:
			want = -mx[i]
		case my[i] > 0:
			want = mx[i]
		}
		if gotMulSign[i] != want {
			panic(16 + i)
		}
	}

	if !archsimd.X86.AVX2() {
		return
	}
	var averageX, averageY [32]uint8
	for i := range averageX {
		averageX[i] = uint8(i * 7)
		averageY[i] = uint8(255 - i*3)
	}
	average256 := composeAverageUnsigned256(archsimd.LoadUint8x32Array(&averageX), archsimd.LoadUint8x32Array(&averageY))
	var gotAverage256 [32]uint8
	average256.StoreArray(&gotAverage256)
	for i := range gotAverage256 {
		want := uint8((uint16(averageX[i]) + uint16(averageY[i]) + 1) / 2)
		if gotAverage256[i] != want {
			panic((32 + i) | int(gotAverage256[i])<<8 | int(want)<<16)
		}
	}

	var highX, highY [16]int16
	for i := range highX {
		highX[i] = int16(i*4095 - 30720)
		highY[i] = int16(28672 - i*3583)
	}
	high256 := composeMulHighSigned256(archsimd.LoadInt16x16Array(&highX), archsimd.LoadInt16x16Array(&highY))
	var gotHigh256 [16]int16
	high256.StoreArray(&gotHigh256)
	for i := range gotHigh256 {
		want := int16((int32(highX[i]) * int32(highY[i])) >> 16)
		if gotHigh256[i] != want {
			panic(64 + i)
		}
	}

	var signX, signY [32]int8
	for i := range signX {
		signX[i] = int8(i*7 - 108)
		signY[i] = int8(i%3 - 1)
	}
	sign256 := composeMulSign256(archsimd.LoadInt8x32Array(&signX), archsimd.LoadInt8x32Array(&signY))
	var gotSign256 [32]int8
	sign256.StoreArray(&gotSign256)
	for i := range gotSign256 {
		want := signX[i] * signY[i]
		if gotSign256[i] != want {
			panic(80 + i)
		}
	}
}
