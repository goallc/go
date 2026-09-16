// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"reflect"
	"unsafe"
)

//go:noinline
func identity(b bool) bool { return b }

// Exhaust both amd64 and arm64 integer argument/result registers.
//
//go:noinline
func many(a, b, c, d, e, f, g, h, i, j, k, l, m, n, o, p, q, r, s, t bool) (bool, bool, bool, bool, bool, bool, bool, bool, bool, bool, bool, bool, bool, bool, bool, bool, bool, bool, bool, bool) {
	return !a, !b, !c, !d, !e, !f, !g, !h, !i, !j, !k, !l, !m, !n, !o, !p, !q, !r, !s, !t
}

//go:noinline
func recovered(b bool) (r bool) {
	defer func() {
		if recover() != nil {
			r = b
		}
	}()
	panic("recover bool")
}

//go:noinline
func address(b bool) bool {
	p := &b
	flip(p)
	return *p
}

//go:noinline
func flip(p *bool) { *p = !*p }

type box struct {
	first byte
	b     bool
	last  byte
}

//go:noinline
func aggregate(b box) (box, bool) { b.b = !b.b; return b, b.b }

func main() {
	for _, b := range []bool{false, true} {
		if identity(b) != b || recovered(b) != b || address(b) == b {
			panic("scalar bool ABI")
		}
		// reflect's native assembly call bridge invokes the LLVM definition and
		// reads its canonical byte result using Go type metadata.
		out := reflect.ValueOf(identity).Call([]reflect.Value{reflect.ValueOf(b)})
		if out[0].Bool() != b {
			panic("native reflect bridge")
		}
		fn := identity
		if fn(b) != b {
			panic("indirect bool ABI")
		}
		r, v := aggregate(box{17, b, 29})
		if r.first != 17 || r.last != 29 || r.b == b || v != r.b {
			panic("aggregate bool ABI")
		}
		byteval := *(*byte)(unsafe.Pointer(&r.b))
		if byteval > 1 {
			panic("noncanonical stored bool")
		}
	}
	a, b, c, d, e, f, g, h, i, j, k, l, m, n, o, p, q, r, s, t := many(false, true, false, true, false, true, false, true, false, true, false, true, false, true, false, true, false, true, false, true)
	result := []bool{a, b, c, d, e, f, g, h, i, j, k, l, m, n, o, p, q, r, s, t}
	for index, v := range result {
		if v != (index%2 == 0) {
			panic("stack bool ABI")
		}
	}
}
