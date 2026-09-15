// run

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"runtime"
	"simd/archsimd"
)

var enabled bool

// No hardware guard here: this ordinary flag must stay an ordinary condition.
// The wide call also exercises vector and pointer live-outs across a GC call.
//
//go:noinline
func calculate(out, x, y *[32]int8) {
	if enabled {
		v, dst := collect(archsimd.LoadInt8x32Array(x), out)
		v.Add(archsimd.LoadInt8x32Array(y)).StoreArray(dst)
	} else {
		*out = [32]int8{99}
	}
}

//go:noinline
func collect(v archsimd.Int8x32, dst *[32]int8) (archsimd.Int8x32, *[32]int8) {
	runtime.GC()
	return v, dst
}

func main() {
	x, y, out := new([32]int8), new([32]int8), new([32]int8)
	calculate(out, x, y) // Must work even when AVX/AVX2 is disabled.
	if *out != [32]int8{99} {
		panic("ordinary false branch lost")
	}
	if !archsimd.X86.AVX2() {
		return
	}
	for i := range x {
		x[i], y[i] = int8(i), 1
	}
	enabled = true
	calculate(out, x, y)
	for i, got := range out {
		if got != int8(i+1) {
			panic("outlined vector or pointer result lost")
		}
	}
	enabled = false
	calculate(out, x, y)
	if *out != [32]int8{99} {
		panic("hardware availability overrode program state")
	}
}
