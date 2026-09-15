// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

var sink uintptr

//go:noinline
func sumPadding(padding *[256]uintptr) uintptr {
	var sum uintptr
	for _, value := range padding {
		sum += value
	}
	return sum
}

// Keep an address-taken frame alive across each recursive call so stack
// growth and copying also occur when the outer caller has no pointer roots.
//
//go:noinline
func grow(depth int) {
	var padding [256]uintptr
	padding[depth&255] = uintptr(depth + 1)
	if depth == 0 {
		runtime.GC()
	} else {
		grow(depth - 1)
	}
	sink += sumPadding(&padding)
}

// Only integers are live across the consecutive void calls. Exercise both
// branch outcomes and a loop after the safepoints without pointer repair.
//
//go:noinline
func exercise(value int, choose bool) int {
	saved := value*3 + 7
	grow(128)
	runtime.GC()
	if choose {
		for i := 0; i < 3; i++ {
			grow(32)
			saved += value + i
		}
	}
	return saved
}

func main() {
	for _, choose := range []bool{false, true} {
		done := make(chan int, 1)
		go func() {
			done <- exercise(11, choose)
		}()
		want := 40
		if choose {
			want += 36
		}
		if got := <-done; got != want {
			panic("integer state lost across empty safepoints")
		}
	}
	// Each grow(n) contributes 1 + ... + (n+1), including after stack copies.
	if sink != 2*(129*130/2)+3*(33*34/2) {
		panic("frame contents lost across stack growth")
	}
}
