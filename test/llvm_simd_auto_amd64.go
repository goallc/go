// run

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"os/exec"
	"runtime"
	"simd/archsimd"
)

var enabled bool
var evaluations int

//go:noinline
func operand(x *[32]int8) *[32]int8 {
	evaluations++
	return x
}

// No hardware guard here: the caller guarantees the ordinary flag is false
// when the CPU cannot execute this path.
// The wide call also exercises vector and pointer results across a GC call.
//
//go:noinline
func calculate(out, x, y *[32]int8) {
	if enabled {
		v, dst := collect(archsimd.LoadInt8x32Array(operand(x)), out)
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
	if len(os.Args) == 1 {
		cmd := exec.Command(os.Args[0], "disabled")
		cmd.Env = append(os.Environ(), "GODEBUG=cpu.avx=off,cpu.avx2=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			panic(string(out) + err.Error())
		}
	}
	x, y, out := new([32]int8), new([32]int8), new([32]int8)
	calculate(out, x, y) // Must work even when AVX/AVX2 is disabled.
	if *out != [32]int8{99} {
		panic("ordinary false branch lost")
	}
	if !archsimd.X86.AVX2() {
		// Violating the CPU precondition is undefined, not a panic fallback.
		return
	}
	for i := range x {
		x[i], y[i] = int8(i), 1
	}
	enabled = true
	calculate(out, x, y)
	if evaluations != 1 {
		panic("operand evaluation lost or duplicated")
	}
	for i, got := range out {
		if got != int8(i+1) {
			panic("automatic SIMD version lost vector or pointer result")
		}
	}
	enabled = false
	calculate(out, x, y)
	if *out != [32]int8{99} {
		panic("hardware availability overrode program state")
	}
}
