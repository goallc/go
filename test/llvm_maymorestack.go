// run -gcflags=-d=maymorestack=main.hook

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

var calls uint64
var noise float64 = 1

// Exercise integer and floating-point registers in the hook. Its caller's
// arguments and closure context must survive before the ordinary prologue.
//
//go:nosplit
func hook() {
	calls++
	noise = noise*0.5 + 1.25
}

//go:noinline
func sum(n, i int, x float64, p *int) float64 {
	if n == 0 {
		return float64(i+*p) + x
	}
	var pad [1024]byte
	pad[0] = byte(n)
	r := sum(n-1, i+3, x+0.5, p)
	runtime.KeepAlive(&pad)
	return r + float64(pad[0]&1)
}

//go:noinline
func makeClosure(seed int, p *int) func(float64) float64 {
	return func(x float64) float64 {
		return sum(128, seed, x, p)
	}
}

func main() {
	p := 11
	f := makeClosure(7, &p)
	if got := f(0.25); got != 530.25 {
		panic("maymorestack corrupted an argument or closure context")
	}
	if calls == 0 {
		panic("maymorestack hook was not called")
	}
}
