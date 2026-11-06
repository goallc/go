// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

// This has the pprof webArgs layout that exposed unaligned fill chunks after
// a bool overwrite. The fill beginning at 113 must not split the pointer at 128.
type args struct {
	Prefix [14]uint64
	Flag   bool
	A, B   *int
	Middle [9]uint64
	P      *int
	N      int
	Q      *int
	M, K   int
	Tail   [10]uint64
}

//go:noinline
func grow(n int) int {
	var a [256]int
	a[n%len(a)] = n
	if n != 0 {
		return grow(n-1) + a[n%len(a)]
	}
	runtime.GC()
	return a[0]
}

//go:noinline
func consume(a args) int {
	grow(64)
	if !a.Flag || a.A != nil || a.B != nil || a.P == nil || a.P != a.Q {
		panic("byval pointer fields")
	}
	for _, v := range a.Prefix {
		if v != 0 {
			panic("byval prefix")
		}
	}
	for _, v := range a.Middle {
		if v != 0 {
			panic("byval middle")
		}
	}
	for _, v := range a.Tail {
		if v != 0 {
			panic("byval tail")
		}
	}
	return *a.P + *a.Q + a.N + a.M + a.K
}

//go:noinline
func construct(run bool, p *int, n int) int {
	runtime.Gosched()
	if run {
		var a args
		a.Flag = true
		a.P, a.N, a.Q, a.M, a.K = p, n, p, n, n
		return consume(a)
	}
	return 0
}

func main() {
	for i := 0; i < 8; i++ {
		p := new(int)
		*p = 7 + i
		if construct(false, p, i) != 0 || construct(true, p, i) != 14+5*i {
			panic("byval result")
		}
	}
}
