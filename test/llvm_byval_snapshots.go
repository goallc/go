// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Exercise complete field initialization without a memset, a partial fill plus
// typed fields, and value snapshots separated from consumption by a mutating
// call. Cross-call initialization may conservatively retain its source object;
// both spellings must preserve the original scalar and pointer across GC and
// observed stack movement. A late reload is an intentionally different value.

package main

import (
	"runtime"
	"unsafe"
)

type node struct{ n uint64 }
type entry struct {
	words [16]uint64
	p     *node
}
type source struct {
	n uint64
	p *node
}
type checker interface{ check(entry) uint64 }
type sink struct{}

var moved int

//go:noinline
func stackAddress(p *byte) uintptr { return uintptr(unsafe.Pointer(p)) }

// Keep the whole recursive stack object live, and observe the address in
// separate calls so optimization cannot fold the relocation comparison.
//
//go:noinline
func sumSpace(a *[1024]byte) byte {
	var sum byte
	for _, v := range a {
		sum += v
	}
	return sum
}

//go:noinline
func grow(n int) byte {
	var a [1024]byte
	a[0] = byte(n)
	if n > 0 {
		a[1] = grow(n - 1)
	}
	return sumSpace(&a)
}

//go:noinline
func (sink) check(e entry) uint64 {
	grow(128)
	runtime.GC()
	sum := e.p.n
	for _, n := range e.words {
		sum += n
	}
	return sum
}

//go:noinline
func mutate(s *source) {
	var marker byte
	before := stackAddress(&marker)
	s.n += 1000
	s.p = &node{99999}
	grow(128)
	if before != stackAddress(&marker) {
		moved++
	}
	runtime.GC()
}

//go:noinline
func snapshotBeforeCall(c checker, s *source) uint64 {
	e := entry{words: [16]uint64{s.n}, p: s.p}
	mutate(s)
	return c.check(e)
}

//go:noinline
func snapshotWithScalars(c checker, s *source) uint64 {
	n, p := s.n, s.p
	mutate(s)
	return c.check(entry{words: [16]uint64{n}, p: p})
}

//go:noinline
func wrongLateSnapshot(c checker, s *source) uint64 {
	mutate(s)
	return c.check(entry{words: [16]uint64{s.n}, p: s.p})
}

//go:noinline
func fullLiteral(c checker, n uint64, p *node) uint64 {
	return c.check(entry{words: [16]uint64{n, n, n, n, n, n, n, n, n, n, n, n, n, n, n, n}, p: p})
}

func main() {
	var c checker = sink{}
	for i := uint64(0); i < 16; i++ {
		done := make(chan bool)
		go func(i uint64) {
			a := source{i, &node{300 + i}}
			b := a
			want := a.n + a.p.n
			if snapshotBeforeCall(c, &a) != want || snapshotWithScalars(c, &b) != want {
				panic("snapshot/GC failure")
			}
			d := source{i, &node{300 + i}}
			if got := wrongLateSnapshot(c, &d); got != i+1000+99999 || got == want {
				panic("negative control failed")
			}
			if fullLiteral(c, i, &node{300 + i}) != 17*i+300 {
				panic("full initialization failed")
			}
			done <- true
		}(i)
		<-done
	}
	if moved == 0 {
		panic("snapshot test did not observe stack relocation")
	}
}
