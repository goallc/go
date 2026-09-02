//go:build amd64 || arm64

// run -goexperiment simd -llvm-package-only

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// Keep dependencies native: enabling the SIMD experiment exposes wider SIMD
// operations that the current LLVM lowering intentionally rejects in runtime
// and archsimd. The command-line package still uses LLVM, matching the scope
// of the former fixture while testing its vector ABI and stack growth.

import (
	"runtime"
	"simd/archsimd"
)

//go:noinline
func llvmVec128TouchFrame(p *[32 << 10]byte) byte {
	p[0] = 1
	p[len(p)-1] = 2
	return p[0] + p[len(p)-1]
}

// llvmVec128AddWithStackGrowth keeps both vector arguments live while using a
// frame larger than a fresh goroutine stack. It exercises the vector argument
// homes used by morestack and an ordinary vector return.
//
//go:noinline
func llvmVec128AddWithStackGrowth(x, y archsimd.Int8x16) archsimd.Int8x16 {
	var frame [32 << 10]byte
	if llvmVec128TouchFrame(&frame) != 3 {
		panic("bad stack frame")
	}
	return x.Add(y)
}

//go:noinline
func llvmVec128CoreIntegerOps(x, y, z archsimd.Int8x16) archsimd.Int8x16 {
	return x.Abs().Sub(y).AndNot(z).Xor(y)
}

//go:noinline
func llvmVec128CoreFloatOps(x, y, z archsimd.Float32x4) archsimd.Float32x4 {
	return x.Mul(y).Sub(z).Div(y).Add(z)
}

//go:noinline
func llvmVec128FloatPhi(useY bool, x, y archsimd.Float32x4) archsimd.Float32x4 {
	selected := x
	if useY {
		selected = y
	}
	return selected.Add(x)
}

//go:noinline
func llvmVec128CoreCompareSelect(x, y archsimd.Int8x16) archsimd.Int8x16 {
	return x.IfElse(x.Greater(y), y)
}

//go:noinline
func llvmVec128CoreFloatNotEqual(x, y archsimd.Float32x4) archsimd.Int32x4 {
	return x.NotEqual(y).ToInt32x4()
}

func main() {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}
	x := [16]int8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	y := [16]int8{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	var got [16]int8
	llvmVec128AddWithStackGrowth(
		archsimd.LoadInt8x16Array(&x),
		archsimd.LoadInt8x16Array(&y),
	).StoreArray(&got)
	for i, value := range got {
		if value != 16 {
			panic(i)
		}
	}

	integerX := [16]int8{-8, -7, -6, -5, -4, -3, -2, -1, 0, 1, 2, 3, 4, 5, 6, 7}
	integerY := [16]int8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	integerZ := [16]int8{0x55, 0x33, 0x0f, 0x55, 0x33, 0x0f, 0x55, 0x33, 0x0f, 0x55, 0x33, 0x0f, 0x55, 0x33, 0x0f, 0x55}
	var integerGot [16]int8
	llvmVec128CoreIntegerOps(
		archsimd.LoadInt8x16Array(&integerX),
		archsimd.LoadInt8x16Array(&integerY),
		archsimd.LoadInt8x16Array(&integerZ),
	).StoreArray(&integerGot)
	for i := range integerGot {
		absX := integerX[i]
		if absX < 0 {
			absX = -absX
		}
		want := ((absX - integerY[i]) &^ integerZ[i]) ^ integerY[i]
		if integerGot[i] != want {
			panic(16 + i)
		}
	}

	floatX := [4]float32{2, 4, 8, 16}
	floatY := [4]float32{2, 2, 4, 4}
	floatZ := [4]float32{2, 4, 8, 16}
	floatWant := [4]float32{3, 6, 14, 28}
	var floatGot [4]float32
	llvmVec128CoreFloatOps(
		archsimd.LoadFloat32x4Array(&floatX),
		archsimd.LoadFloat32x4Array(&floatY),
		archsimd.LoadFloat32x4Array(&floatZ),
	).StoreArray(&floatGot)
	for i := range floatGot {
		if floatGot[i] != floatWant[i] {
			panic(32 + i)
		}
	}

	floatPhiWant := [4]float32{4, 6, 12, 20}
	llvmVec128FloatPhi(
		true,
		archsimd.LoadFloat32x4Array(&floatX),
		archsimd.LoadFloat32x4Array(&floatY),
	).StoreArray(&floatGot)
	for i := range floatGot {
		if floatGot[i] != floatPhiWant[i] {
			panic(40 + i)
		}
	}

	selectX := [16]int8{-8, 7, -6, 5, -4, 3, -2, 1, 0, -1, 2, -3, 4, -5, 6, -7}
	selectY := [16]int8{8, -7, 6, -5, 4, -3, 2, -1, 0, 1, -2, 3, -4, 5, -6, 7}
	var selectGot [16]int8
	llvmVec128CoreCompareSelect(
		archsimd.LoadInt8x16Array(&selectX),
		archsimd.LoadInt8x16Array(&selectY),
	).StoreArray(&selectGot)
	for i := range selectGot {
		want := selectX[i]
		if selectY[i] > want {
			want = selectY[i]
		}
		if selectGot[i] != want {
			panic(48 + i)
		}
	}

	zero := float32(0)
	nan := zero / zero
	compareX := [4]float32{nan, 1, nan, 2}
	compareY := [4]float32{nan, 1, 3, nan}
	compareWant := [4]int32{-1, 0, -1, -1}
	var compareGot [4]int32
	llvmVec128CoreFloatNotEqual(
		archsimd.LoadFloat32x4Array(&compareX),
		archsimd.LoadFloat32x4Array(&compareY),
	).StoreArray(&compareGot)
	for i := range compareGot {
		if compareGot[i] != compareWant[i] {
			panic(64 + i)
		}
	}
}
