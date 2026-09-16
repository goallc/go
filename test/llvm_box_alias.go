// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

type words struct{ a, b, c uintptr }
type pointers struct{ p, q *int }

//go:noinline
func boxWords(x words) any { return x }

//go:noinline
func boxPointers(x pointers) any { return x }

func main() {
	x := words{1, 2, 3}
	a := boxWords(x)
	x.a = 11
	b := boxWords(x)
	if a.(words).a != 1 || b.(words).a != 11 {
		panic("box storage aliases source")
	}
	n, m := 7, 9
	p := pointers{&n, &m}
	c := boxPointers(p)
	p.p = &m
	d := boxPointers(p)
	n = 17
	runtime.GC()
	if c.(pointers).p != &n || *c.(pointers).p != 17 || d.(pointers).p != &m {
		panic("pointer fields lost original alias")
	}
	c1, c2 := make(chan struct{}), make(chan struct{})
	if c1 == c2 {
		panic("channel headers alias")
	}
	m1, m2 := make(map[int]int), make(map[int]int)
	m1[1] = 2
	if m2[1] != 0 {
		panic("map headers alias")
	}
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	runtime.KeepAlive(c)
	runtime.KeepAlive(d)
}
