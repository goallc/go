// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

type large [1 << 16]byte

var escaped *large
var escapedPointer **int

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

// A later escaped result inserts VarDef between the aggregate read and return,
// even without race instrumentation.
//
//go:noinline
func multipleResults() (r large, p *int) {
	escaped, escapedPointer = &r, &p
	fill(&r)
	p = new(int)
	*p = 42
	return
}

// Copies and clears of named results remain observable through recovery.
//
//go:noinline
func recoveryResult(src *large, clearResult bool) (r large) {
	defer func() { recover() }()
	r = *src
	if clearResult {
		r = large{}
	}
	panic("return through recover")
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
	multi, p := multipleResults()
	check(multi)
	if *p != 42 {
		panic("second result changed")
	}
	r := heapResult()
	escaped[0] = 99
	runtime.GC()
	check(r)
	// A byval argument must retain the value read before mutating its source.
	fill(escaped)
	check(recoveryResult(escaped, false))
	if recoveryResult(escaped, true) != (large{}) {
		panic("clear lost during recovery")
	}
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
