// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

// grow forces repeated stack growth while pointer is both an entry argument
// and live across the recursive safepoint. At depth zero, runtime.GC scans the
// copied stack before the recursion unwinds.
//
//go:noinline
func grow(pointer *int, depth int) int {
	if depth == 0 {
		runtime.GC()
		return *pointer
	}
	return grow(pointer, depth-1) + *pointer
}

func main() {
	value := 91
	const depth = 2000
	if got, want := grow(&value, depth), value*(depth+1); got != want {
		panic("pointer argument lost across stack growth or GC")
	}
}
