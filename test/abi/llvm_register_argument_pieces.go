// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

type strings3 struct {
	a, b, c string
}

type strings2Array [2]string

type mixedRegisters struct {
	a, b          int16
	c, d, e       int32
	r, s, t, u, v float32
}

//go:noinline
func stringLengths(x strings3) int {
	return len(x.a) + len(x.b) + len(x.c)
}

//go:noinline
func arrayStringLengths(x strings2Array) int {
	return len(x[0]) + len(x[1])
}

//go:noinline
func touchFloat(*float32) {}

//go:noinline
func moveLastFloat(x mixedRegisters) mixedRegisters {
	y := x.v
	touchFloat(&y)
	x.r = y
	return x
}

//go:noinline
func emptySliceInterface() int {
	x := any([]byte{})
	return len(x.([]byte))
}

func main() {
	if stringLengths(strings3{"ab", "cde", "fghi"}) != 9 {
		panic("bad string register pieces")
	}
	if arrayStringLengths(strings2Array{"abc", "defgh"}) != 8 {
		panic("bad stack argument home")
	}

	x := mixedRegisters{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	got := moveLastFloat(x)
	want := mixedRegisters{1, 2, 3, 4, 5, 10, 7, 8, 9, 10}
	if got != want {
		panic("bad mixed register pieces")
	}
	if emptySliceInterface() != 0 {
		panic("bad empty value")
	}
}
