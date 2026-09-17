// run -gcflags='-N -l'

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

//go:noinline
func lookup(x int) (int, bool) {
	return x + 3, x > 0
}

//go:noinline
func choose(x int) int {
	v, ok := lookup(x)
	if ok {
		return v
	}
	return -v
}

//go:noinline
func nine() (int, int, int, int, int, int, int, int, int) {
	return 1, 2, 3, 4, 5, 6, 7, 8, 9
}

func main() {
	if choose(-2) != -1 || choose(0) != -3 || choose(7) != 10 {
		panic("call result condition")
	}
	a, b, c, d, e, f, g, h, i := nine()
	if a != 1 || b != 2 || c != 3 || d != 4 || e != 5 || f != 6 || g != 7 || h != 8 || i != 9 {
		panic("call result registers")
	}
}
