//go:build amd64

// run -goexperiment simd -llvm-package-only

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"simd/archsimd"
)

//go:noinline
func guardedAdd(x, y archsimd.Int8x32) archsimd.Int8x32 {
	if !archsimd.X86.AVX512() {
		return x
	}
	return x.Add(y)
}

//go:noinline
func nestedAdd(x, y archsimd.Int8x32) (archsimd.Int8x32, bool) {
	if archsimd.X86.AVX512() {
		if archsimd.X86.AVX2() {
			return x.Add(y), true
		}
		return x.Add(x), false
	}
	if archsimd.X86.AVX2() {
		return y.Add(y), true
	}
	return x, false
}

//go:noinline
func subsetAdd(x, y archsimd.Int8x32) archsimd.Int8x32 {
	if archsimd.X86.AVX512BITALG() {
		return x.Add(y)
	}
	if archsimd.X86.AVX512VPOPCNTDQ() {
		return y.Add(y)
	}
	return x
}

// Exercise VPOPCNTDQ even on CPUs that also have BITALG.
//
//go:noinline
func vpopAdd(x, y archsimd.Int8x32) archsimd.Int8x32 {
	if archsimd.X86.AVX512VPOPCNTDQ() {
		return x.Add(y)
	}
	return x
}

//go:noinline
func subsetGuardsAVX512(x, y, out *[64]int8) {
	if archsimd.X86.AVX512BITALG() {
		a := archsimd.LoadInt8x64Array(x)
		b := archsimd.LoadInt8x64Array(y)
		a.Add(b).StoreArray(out)
		return
	}
	*out = *x
}

// A scalar signature prevents the vector ABI from supplying the AVX floor.
//
//go:noinline
func avx2GuardsAVX(x, y *[8]float32, out *[8]float32) bool {
	if archsimd.X86.AVX2() {
		a := archsimd.LoadFloat32x8Array(x)
		b := archsimd.LoadFloat32x8Array(y)
		a.Add(b).StoreArray(out)
		return archsimd.X86.AVX()
	}
	*out = *x
	return archsimd.X86.AVX()
}

func main() {
	avx, avx2, avx512 := archsimd.X86.AVX(), archsimd.X86.AVX2(), archsimd.X86.AVX512()
	bitalg, vpop := archsimd.X86.AVX512BITALG(), archsimd.X86.AVX512VPOPCNTDQ()
	var a, b, out [8]float32
	for i := range a {
		a[i], b[i] = float32(i+1), 3
	}
	if avx2GuardsAVX(&a, &b, &out) != avx {
		panic("changed effective AVX boolean")
	}
	for i, got := range out {
		want := a[i]
		if avx2 {
			want += b[i]
		}
		if got != want {
			panic("AVX2 guarding AVX")
		}
	}
	// Enter the wide ABI only when a real feature establishes its hardware
	// contract, including when the lower effective boolean is disabled.
	if avx || avx2 || avx512 || bitalg || vpop {
		checkWide(avx2, avx512, bitalg, vpop)
	}
	var wideX, wideY, wideOut [64]int8
	for i := range wideX {
		wideX[i], wideY[i] = int8(i), 2
	}
	subsetGuardsAVX512(&wideX, &wideY, &wideOut)
	for i, got := range wideOut {
		want := wideX[i]
		if bitalg {
			want += wideY[i]
		}
		if got != want {
			panic("BITALG guarding AVX512")
		}
	}
	if os.Getenv("GOALLC_STRONG_GUARDS_PRINT") == "1" {
		fmt.Printf("AVX=%v AVX2=%v AVX512=%v BITALG=%v VPOPCNTDQ=%v OK\n", avx, avx2, avx512, bitalg, vpop)
	}
}

//go:noinline
func checkWide(avx2, avx512, bitalg, vpop bool) {
	var x, y, got [32]int8
	for i := range x {
		x[i], y[i] = int8(i+1), 3
	}
	vx, vy := archsimd.LoadInt8x32Array(&x), archsimd.LoadInt8x32Array(&y)
	guardedAdd(vx, vy).StoreArray(&got)
	for i := range got {
		want := x[i]
		if avx512 {
			want += y[i]
		}
		if got[i] != want {
			panic("AVX512 guarding AVX2")
		}
	}
	v, effective := nestedAdd(vx, vy)
	if effective != avx2 {
		panic("changed effective AVX2 boolean")
	}
	v.StoreArray(&got)
	for i := range got {
		want := x[i]
		switch {
		case avx512 && avx2:
			want += y[i]
		case avx512:
			want += x[i]
		case avx2:
			want = y[i] + y[i]
		}
		if got[i] != want {
			panic("nested independent booleans")
		}
	}
	subsetAdd(vx, vy).StoreArray(&got)
	for i := range got {
		want := x[i]
		if bitalg {
			want += y[i]
		} else if vpop {
			want = y[i] + y[i]
		}
		if got[i] != want {
			panic("AVX512 subset guarding AVX2")
		}
	}
	vpopAdd(vx, vy).StoreArray(&got)
	for i := range got {
		want := x[i]
		if vpop {
			want += y[i]
		}
		if got[i] != want {
			panic("VPOPCNTDQ guarding AVX2")
		}
	}
}
