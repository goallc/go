// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

type pair struct {
	a int
	b *int
}

//go:noinline
func observe(p *int) {
	if *p == -1 {
		panic("unreachable")
	}
}

func inner(x int, p *pair) int {
	local := x + p.a
	observe(&local)
	return local
}

func middle(x int, p *pair) int {
	return inner(x, p)
}

func outer(x int, p *pair) int {
	return middle(x, p)
}

func main() {
	p := pair{a: 4}
	if outer(3, &p) != 7 {
		panic("bad result")
	}
	lo, hi := split(9)
	if lo != 9 || hi != 10 {
		panic("bad split")
	}
	words := [1]uintptr{11}
	if arrayWord(&words) != 11 {
		panic("bad word")
	}
}

//go:noinline
func split(x int) (lo, hi int) {
	return x, x + 1
}

//go:noinline
func arrayWord(p *[1]uintptr) uintptr {
	return p[0]
}
