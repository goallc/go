// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

//go:noinline
func overwrittenCopy(p *[8]uint64, a, b uint64) {
	t := [8]uint64{a, a, a, a, a, a, a, a}
	*p = t
	p[0] = b
	p[1] = b
	p[2] = b
	p[3] = b
	p[4] = b
	p[5] = b
	p[6] = b
	p[7] = b
}

//go:noinline
func partialCopy(p *[8]uint64, a, b uint64) {
	t := [8]uint64{a, a, a, a, a, a, a, a}
	*p = t
	p[0] = b
}

//go:noinline
func observe(p *[8]uint64) [8]uint64 { return *p }

//go:noinline
func observedCopy(p *[8]uint64, a, b uint64) [8]uint64 {
	t := [8]uint64{a, a, a, a, a, a, a, a}
	*p = t
	x := observe(p)
	overwrittenCopy(p, a, b)
	return x
}

func checkNil() {
	defer func() {
		if recover() == nil {
			panic("lost nil destination panic")
		}
	}()
	overwrittenCopy(nil, 1, 2)
}

func main() {
	for i := uint64(0); i < 16; i++ {
		a, b := i, ^i
		var p [8]uint64
		overwrittenCopy(&p, a, b)
		if p != [8]uint64{b, b, b, b, b, b, b, b} {
			panic("wrong fully overwritten copy")
		}
		partialCopy(&p, a, b)
		if p != [8]uint64{b, a, a, a, a, a, a, a} {
			panic("lost partially overwritten copy")
		}
		x := observedCopy(&p, a, b)
		if x != [8]uint64{a, a, a, a, a, a, a, a} || p != [8]uint64{b, b, b, b, b, b, b, b} {
			panic("lost copy observed by a call")
		}
	}
	checkNil()
}
