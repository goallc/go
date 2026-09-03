//go:build amd64 || arm64

// run -goexperiment simd -llvm-package-only

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"math"
	"runtime"
	"simd/archsimd"
)

//go:noinline
func standardCeil(x archsimd.Float32x4) archsimd.Float32x4 {
	return x.Ceil()
}

//go:noinline
func standardFloor(x archsimd.Float32x4) archsimd.Float32x4 {
	return x.Floor()
}

//go:noinline
func standardRound(x archsimd.Float32x4) archsimd.Float32x4 {
	return x.Round()
}

//go:noinline
func standardSqrt(x archsimd.Float32x4) archsimd.Float32x4 {
	return x.Sqrt()
}

//go:noinline
func standardTrunc(x archsimd.Float32x4) archsimd.Float32x4 {
	return x.Trunc()
}

//go:noinline
func standardIntMinMax(x, y archsimd.Int16x8) (archsimd.Int16x8, archsimd.Int16x8) {
	return x.Min(y), x.Max(y)
}

//go:noinline
func standardUintMinMax(x, y archsimd.Uint32x4) (archsimd.Uint32x4, archsimd.Uint32x4) {
	return x.Min(y), x.Max(y)
}

//go:noinline
func standardFloatMinMax(x, y archsimd.Float32x4) (archsimd.Float32x4, archsimd.Float32x4) {
	return x.Min(y), x.Max(y)
}

// Keep the source-level feature checks in the same functions as the optional
// amd64 operations. This exercises their CPU profiles and the early FMV pass;
// arm64 reaches the standard LLVM intrinsics directly.

//go:noinline
func standardLeadingZeros(x archsimd.Uint32x4) archsimd.Uint32x4 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return x
	}
	return x.LeadingZeros()
}

//go:noinline
func standardOnesCount(x archsimd.Uint8x16) archsimd.Uint8x16 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512BITALG() {
		return x
	}
	return x.OnesCount()
}

func checkFloat32x4(got archsimd.Float32x4, want [4]float32, code int) {
	var values [4]float32
	got.StoreArray(&values)
	for i := range values {
		if values[i] != want[i] {
			panic(code + i)
		}
	}
}

func main() {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}

	roundingInput := [4]float32{2.5, -2.5, 3.75, -3.75}
	rounding := archsimd.LoadFloat32x4Array(&roundingInput)
	checkFloat32x4(standardCeil(rounding), [4]float32{3, -2, 4, -3}, 0)
	checkFloat32x4(standardFloor(rounding), [4]float32{2, -3, 3, -4}, 4)
	checkFloat32x4(standardRound(rounding), [4]float32{2, -2, 4, -4}, 8)
	checkFloat32x4(standardTrunc(rounding), [4]float32{2, -2, 3, -3}, 12)
	sqrtInput := [4]float32{0, 1, 4, 9}
	checkFloat32x4(standardSqrt(archsimd.LoadFloat32x4Array(&sqrtInput)), [4]float32{0, 1, 2, 3}, 16)

	intsX := [8]int16{-8, 7, -6, 5, -4, 3, -2, 1}
	intsY := [8]int16{8, -7, 6, -5, 4, -3, 2, -1}
	intMin, intMax := standardIntMinMax(
		archsimd.LoadInt16x8Array(&intsX),
		archsimd.LoadInt16x8Array(&intsY),
	)
	var gotIntMin, gotIntMax [8]int16
	intMin.StoreArray(&gotIntMin)
	intMax.StoreArray(&gotIntMax)
	for i := range gotIntMin {
		wantMin, wantMax := intsX[i], intsY[i]
		if wantMin > wantMax {
			wantMin, wantMax = wantMax, wantMin
		}
		if gotIntMin[i] != wantMin || gotIntMax[i] != wantMax {
			panic(24 + i)
		}
	}

	uintsX := [4]uint32{0, 10, 1 << 31, ^uint32(0)}
	uintsY := [4]uint32{1, 9, 1<<31 - 1, 42}
	uintMin, uintMax := standardUintMinMax(
		archsimd.LoadUint32x4Array(&uintsX),
		archsimd.LoadUint32x4Array(&uintsY),
	)
	var gotUintMin, gotUintMax [4]uint32
	uintMin.StoreArray(&gotUintMin)
	uintMax.StoreArray(&gotUintMax)
	for i := range gotUintMin {
		wantMin, wantMax := uintsX[i], uintsY[i]
		if wantMin > wantMax {
			wantMin, wantMax = wantMax, wantMin
		}
		if gotUintMin[i] != wantMin || gotUintMax[i] != wantMax {
			panic(32 + i)
		}
	}

	nan := math.Float32frombits(0x7fc00001)
	negativeZero := math.Float32frombits(1 << 31)
	floatsX := [4]float32{nan, negativeZero, 7, -8}
	floatsY := [4]float32{1, 0, -7, 8}
	floatMin, floatMax := standardFloatMinMax(
		archsimd.LoadFloat32x4Array(&floatsX),
		archsimd.LoadFloat32x4Array(&floatsY),
	)
	var gotFloatMin, gotFloatMax [4]float32
	floatMin.StoreArray(&gotFloatMin)
	floatMax.StoreArray(&gotFloatMax)
	if !math.IsNaN(float64(gotFloatMin[0])) || !math.IsNaN(float64(gotFloatMax[0])) ||
		!math.Signbit(float64(gotFloatMin[1])) || math.Signbit(float64(gotFloatMax[1])) ||
		gotFloatMin[2] != -7 || gotFloatMax[2] != 7 ||
		gotFloatMin[3] != -8 || gotFloatMax[3] != 8 {
		panic(36)
	}

	leadingInput := [4]uint32{0, 1, 0x00f00000, 1 << 31}
	leading := standardLeadingZeros(archsimd.LoadUint32x4Array(&leadingInput))
	var gotLeading [4]uint32
	leading.StoreArray(&gotLeading)
	if runtime.GOARCH != "amd64" || archsimd.X86.AVX512() {
		wantLeading := [4]uint32{32, 31, 8, 0}
		for i := range gotLeading {
			if gotLeading[i] != wantLeading[i] {
				panic(40 + i)
			}
		}
	} else if gotLeading != leadingInput {
		panic(44)
	}

	countInput := [16]uint8{0, 1, 2, 3, 7, 8, 15, 16, 31, 63, 127, 128, 129, 170, 240, 255}
	counted := standardOnesCount(archsimd.LoadUint8x16Array(&countInput))
	var gotCount [16]uint8
	counted.StoreArray(&gotCount)
	if runtime.GOARCH != "amd64" || archsimd.X86.AVX512BITALG() {
		wantCount := [16]uint8{0, 1, 1, 2, 3, 1, 4, 1, 5, 6, 7, 1, 2, 4, 4, 8}
		for i := range gotCount {
			if gotCount[i] != wantCount[i] {
				panic(48 + i)
			}
		}
	} else if gotCount != countInput {
		panic(64)
	}

}
