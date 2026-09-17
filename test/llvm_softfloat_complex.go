// run -gcflags=-d=softfloat

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

var real32, imag32 float32 = 1.25, -2.5
var real64, imag64 float64 = -3.75, 4.5

//go:noinline
func make64(dst *complex64, r, i *float32) { *dst = complex(*r, *i) }

//go:noinline
func make128(dst *complex128, r, i *float64) { *dst = complex(*r, *i) }

//go:noinline
func parts64(c *complex64, r, i *float32) { *r, *i = real(*c), imag(*c) }

//go:noinline
func parts128(c *complex128, r, i *float64) { *r, *i = real(*c), imag(*c) }

//go:noinline
func check64(c complex64) bool { return real(c) == real32 && imag(c) == imag32 }

//go:noinline
func check128(c complex128) bool { return real(c) == real64 && imag(c) == imag64 }

func main() {
	if !check64(complex(real32, imag32)) || !check128(complex(real64, imag64)) {
		panic("complex argument changed float bits")
	}
	var c64 complex64
	var c128 complex128
	make64(&c64, &real32, &imag32)
	make128(&c128, &real64, &imag64)
	if c64 != complex(real32, imag32) || c128 != complex(real64, imag64) {
		panic("complex construction changed float bits")
	}
	var r32, i32 float32
	var r64, i64 float64
	parts64(&c64, &r32, &i32)
	parts128(&c128, &r64, &i64)
	if r32 != real32 || i32 != imag32 || r64 != real64 || i64 != imag64 {
		panic("complex component round trip")
	}
	if c64+complex64(c128) != complex64(-2.5+2i) {
		panic("complex arithmetic")
	}
}
