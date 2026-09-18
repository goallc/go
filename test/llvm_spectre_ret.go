// run -gcflags=-spectre=all

//go:build amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

type call9 func(int, int, int, int, int, int, int, int, int) int

//go:noinline
func invoke(f call9) int { return f(1, 2, 3, 4, 5, 6, 7, 8, 9) }

//go:noinline
func weighted(a, b, c, d, e, f, g, h, i int) int {
	runtime.GC()
	return a + 2*b + 3*c + 4*d + 5*e + 6*f + 7*g + 8*h + 9*i
}

//go:noinline
func closure(x *int) call9 {
	return func(a, b, c, d, e, f, g, h, i int) int {
		r := weighted(a, b, c, d, e, f, g, h, i)
		return r + *x
	}
}

type method9 interface {
	Sum(int, int, int, int, int, int, int, int, int) int
}

type receiver struct{ n int }

func (r *receiver) Sum(a, b, c, d, e, f, g, h, i int) int {
	return weighted(a, b, c, d, e, f, g, h, i) + r.n
}

//go:noinline
func invokeMethod(m method9) int { return m.Sum(1, 2, 3, 4, 5, 6, 7, 8, 9) }

//go:noinline
func invokeDeferred(f call9) (r int) {
	defer func() { r = invoke(f) }()
	return
}

func main() {
	if invoke(weighted) != 285 {
		panic("retpoline changed register arguments")
	}
	x := 17
	f := closure(&x)
	if invoke(f) != 302 || invokeDeferred(f) != 302 {
		panic("retpoline changed closure context")
	}
	r := &receiver{23}
	if invokeMethod(r) != 308 || invoke(r.Sum) != 308 {
		panic("retpoline changed method arguments")
	}
	// Exercise an index mask together with indirect calls under -spectre=all.
	fs := []call9{weighted, f, r.Sum}
	for i, want := range []int{285, 302, 308} {
		if invoke(fs[i]) != want {
			panic("combined index and ret mitigation changed result")
		}
	}
}
