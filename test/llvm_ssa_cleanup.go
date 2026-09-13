// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

//go:noinline
func observedStore(p, q *[4]uint64, a, b uint64) uint64 {
	p[0] = a
	x := q[0]
	p[0] = b
	return x
}

//go:noinline
func partialZero(p *[4]uint64, v uint64) {
	*p = [4]uint64{}
	p[0] = v
}

//go:noinline
func readAfterGC(p *[2]*int) int {
	runtime.GC()
	return *p[0] + *p[1]
}

//go:noinline
func pointerStores(p *[2]*int, a, b *int) int {
	*p = [2]*int{a, b}
	x := readAfterGC(p)
	*p = [2]*int{b, a}
	return x
}

//go:noinline
func fusedLoop(n int) int {
	x := 0
	for i := 0; i < n; i++ {
		if x == 0 {
			x = 0
		} else {
			x = 1
		}
	}
	return x
}

func main() {
	for i := 0; i < 16; i++ {
		p := [4]uint64{1, 2, 3, 4}
		q := [4]uint64{9, 8, 7, 6}
		if observedStore(&p, &p, 11, 12) != 11 || p[0] != 12 {
			panic("lost store before aliasing read")
		}
		if observedStore(&p, &q, 13, 14) != 9 || p[0] != 14 {
			panic("changed non-aliasing read")
		}
		partialZero(&p, uint64(i))
		if p != [4]uint64{uint64(i), 0, 0, 0} {
			panic("lost partially overwritten zero")
		}
		a, b := new(int), new(int)
		*a, *b = i, i+1
		ptrs := new([2]*int)
		if pointerStores(ptrs, a, b) != 2*i+1 || ptrs[0] != b || ptrs[1] != a {
			panic("lost pointer store across call and GC")
		}
		if fusedLoop(i-1) != 0 {
			panic("changed fused loop result")
		}
	}
}
