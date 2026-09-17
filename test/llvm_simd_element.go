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
func setGetInt32(x archsimd.Int32x4, index uint8, value int32) (archsimd.Int32x4, int32) {
	x = x.SetElem(index, value)
	return x, x.GetElem(index)
}

//go:noinline
func setGetFloat64(x archsimd.Float64x2, index uint8, value float64) (archsimd.Float64x2, float64) {
	x = x.SetElem(index, value)
	return x, x.GetElem(index)
}

func mustPanic(f func()) {
	didPanic := false
	func() {
		defer func() {
			didPanic = recover() != nil
		}()
		f()
	}()
	if !didPanic {
		panic("missing SIMD element-index panic")
	}
}

func main() {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}

	input := [4]int32{10, 20, 30, 40}
	for index := uint8(0); index < 4; index++ {
		value := int32(100 + index)
		updated, got := setGetInt32(archsimd.LoadInt32x4Array(&input), index, value)
		var lanes [4]int32
		updated.StoreArray(&lanes)
		for i := range lanes {
			want := input[i]
			if uint8(i) == index {
				want = value
			}
			if lanes[i] != want {
				panic(i)
			}
		}
		if got != value {
			panic(got)
		}
	}

	floats := [2]float64{1.25, -2.5}
	updated, got := setGetFloat64(archsimd.LoadFloat64x2Array(&floats), 1, 9.5)
	if got != 9.5 || updated.GetElem(0) != floats[0] {
		panic(got)
	}

	vector := archsimd.LoadInt32x4Array(&input)
	mustPanic(func() { vector.GetElem(4) })
	mustPanic(func() { vector.SetElem(4, 0) })
}
