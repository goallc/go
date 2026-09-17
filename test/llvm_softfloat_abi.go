// run -gcflags=-d=softfloat

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

type pair struct {
	f float64
	i int
}

var f32 float32 = -1.25
var f64 float64 = 3.5
var c64 complex64 = 2 - 4i
var c128 complex128 = -5 + 6i
var aggregate = pair{-7.5, 42}
var indirect = roundtrip

//go:noinline
func roundtrip(f float32, d float64, c complex64, z complex128, s pair) (float32, int, float64, complex64, complex128, pair) {
	return f, s.i, d, c, z, s
}

//go:noinline
func check(f float32, n int, d float64, c complex64, z complex128, s pair) {
	if f != f32 || n != aggregate.i || d != f64 || c != c64 || z != c128 || s != aggregate {
		panic("soft-float ABI round trip")
	}
}

func main() {
	check(roundtrip(f32, f64, c64, c128, aggregate))
	check(indirect(f32, f64, c64, c128, aggregate))
}
