// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

var markers = []string{"<builtin.", "<builtin.42>", "<linkname>", "<goallc.fmv.baseline>", "<ABI0>"}

//go:noinline
func marker(i int) string {
	switch i {
	case 0:
		return "<builtin."
	case 1:
		return "<builtin.42>"
	case 2:
		return "<linkname>"
	case 3:
		return "<goallc.fmv.baseline>"
	default:
		return "<ABI0>"
	}
}

func main() {
	for i, want := range markers {
		if got := marker(i); got != want {
			panic("symbol suffix text changed")
		}
	}
}
