// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

// These aggregates occupy one pointer word despite having multiple fields.
// Interface operations can forward either the whole aggregate or its pointer
// leaf into function results and write-barrier stores.
type pointerCarrier struct {
	a, b, c, d struct{}
	p          *int
}

type nestedCarrier struct {
	zero [2][0]uint64
	p    [1]pointerCarrier
}

type mapCarrier struct {
	m map[int]int
}

var root any
var maps = make(map[int]any)

//go:noinline
func publish(v any) {
	root = v
}

//go:noinline
func box(p *int, nested bool) {
	v := pointerCarrier{p: p}
	if nested {
		publish(nestedCarrier{p: [1]pointerCarrier{v}})
	} else {
		publish(v)
	}
}

//go:noinline
func unbox() pointerCarrier {
	return root.(pointerCarrier)
}

//go:noinline
func unboxNested() nestedCarrier {
	return root.(nestedCarrier)
}

//go:noinline
func makeMap() mapCarrier {
	return mapCarrier{map[int]int{7: 42}}
}

//go:noinline
func publishMap(v mapCarrier) {
	root = v
	maps[0] = v
}

//go:noinline
func choose(x, y pointerCarrier, first bool) pointerCarrier {
	if first {
		return x
	}
	return y
}

func main() {
	x, y := 42, 99
	for _, nested := range []bool{false, true} {
		box(&x, nested)
		runtime.GC()
		var p *int
		if nested {
			p = unboxNested().p[0].p
		} else {
			p = unbox().p
		}
		if p != &x || *p != 42 {
			panic("interface pointer carrier changed")
		}
		box(nil, nested)
		if nested && unboxNested().p[0].p != nil || !nested && unbox().p != nil {
			panic("nil interface pointer carrier changed")
		}
	}
	for _, first := range []bool{false, true} {
		v := choose(pointerCarrier{p: &x}, pointerCarrier{p: &y}, first)
		want := &y
		if first {
			want = &x
		}
		if v.p != want {
			panic("aggregate result changed")
		}
	}
	publishMap(makeMap())
	runtime.GC()
	if root.(mapCarrier).m[7] != 42 || maps[0].(mapCarrier).m[7] != 42 {
		panic("interface map carrier changed")
	}
	root.(mapCarrier).m[7] = 99
	if maps[0].(mapCarrier).m[7] != 99 {
		panic("interface map identity changed")
	}
}
