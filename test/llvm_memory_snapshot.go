// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

type large [1 << 16]byte

var escaped *large

//go:noinline
func fill(r *large) {
	for i := range r {
		r[i] = byte(i)
	}
}

// With race instrumentation, the result read precedes racefuncexit, while
// the ABI result store follows it. The value must remain a memory snapshot.
//
//go:noinline
func stackResult() (r large) {
	fill(&r)
	return
}

//go:noinline
func heapResult() (r large) {
	escaped = &r
	fill(&r)
	return
}

//go:noinline
func check(r large) {
	for i, v := range r {
		if v != byte(i) {
			panic("large result changed")
		}
	}
}

//go:noinline
func mutate(r *large) {
	r[42] = 99
}

//go:noinline
func pointerResult() (r [32]*int) {
	for i := range r {
		x := i
		r[i] = &x
	}
	return
}

func main() {
	check(stackResult())
	r := heapResult()
	escaped[0] = 99
	runtime.GC()
	check(r)
	// A byval argument must retain the value read before mutating its source.
	fill(escaped)
	snapshot := *escaped
	mutate(escaped)
	check(snapshot)
	pointers := pointerResult()
	runtime.GC()
	for i, p := range pointers {
		if *p != i {
			panic("pointer result changed after GC")
		}
	}
}
