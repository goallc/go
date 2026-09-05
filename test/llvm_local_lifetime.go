// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

type record struct {
	p *int
	x [4]int
}

//go:noinline
func grow(n int) byte {
	var pad [1024]byte
	pad[n%len(pad)] = byte(n)
	if n != 0 {
		pad[0] = grow(n - 1)
	}
	return pad[n%len(pad)]
}

//go:noinline
func observe(r *record, want int) {
	grow(64)
	runtime.GC()
	if r.x[0] != want || (r.p != nil && *r.p != 42) {
		panic("local lifetime corrupted")
	}
	r.x[0]++
}

//go:noinline
func branch(take bool) {
	// The address-taken local's StackObject exists in this frame, but dead
	// contents need not be initialized merely for conservative stack adjustment.
	grow(64)
	runtime.GC()
	if take {
		var r record
		observe(&r, 0)
		r = record{}
		observe(&r, 0)
	}
}

func main() {
	branch(false)
	branch(true)
	n := 42
	var carried record
	carried.p = &n
	for i := 0; i < 8; i++ {
		observe(&carried, i)
		var fresh record
		observe(&fresh, 0)
		fresh.p = &n
		observe(&fresh, 1)
	}
	if carried.x[0] != 8 {
		panic("lost loop-carried contents")
	}
}
