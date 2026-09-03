//go:build amd64

// run -goexperiment simd -llvm-package-only

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

//go:noinline
func standardOnesCount32(x archsimd.Uint32x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVX512VPOPCNTDQ() {
		return x
	}
	return x.OnesCount()
}

func main() {
	input := [4]uint32{0, 1, 0xf0f00000, ^uint32(0)}
	counted := standardOnesCount32(archsimd.LoadUint32x4Array(&input))
	var got [4]uint32
	counted.StoreArray(&got)
	if archsimd.X86.AVX512VPOPCNTDQ() {
		want := [4]uint32{0, 1, 8, 32}
		if got != want {
			panic("incorrect AVX-512 VPOPCNTDQ result")
		}
	} else if got != input {
		panic("incorrect AVX-512 VPOPCNTDQ fallback")
	}
}
