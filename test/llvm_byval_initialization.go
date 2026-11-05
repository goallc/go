// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

type node struct{ value uint64 }

// 136 bytes on 64-bit targets, with pointers at both ends of the copied area.
type entry struct {
	level byte
	pad   [7]byte
	first *node
	zero  [14]uint64
	last  *node
}

type checker interface {
	check(entry, int) uint64
}

type sink struct{}

//go:noinline
func grow(n int) byte {
	var space [1024]byte
	space[0] = byte(n)
	if n != 0 {
		space[1] = grow(n - 1)
	}
	runtime.KeepAlive(&space)
	return space[0] + space[1]
}

//go:noinline
func (sink) check(e entry, tag int) uint64 {
	grow(24)
	runtime.GC()
	for _, v := range e.pad {
		if v != 0 {
			panic("nonzero padding field")
		}
	}
	for _, v := range e.zero {
		if v != 0 {
			panic("nonzero field")
		}
	}
	return uint64(e.level) + e.first.value + e.last.value + uint64(tag)
}

//go:noinline
func send(c checker, level byte, a, b *node) uint64 {
	return c.check(entry{level: level, first: a, last: b}, 0)
}

var effects int

//go:noinline
func argument() int {
	effects++
	grow(24)
	runtime.GC()
	return effects
}

//go:noinline
func sendWithEffects(c checker, level byte, a, b *node) uint64 {
	return c.check(entry{level: level, first: a, last: b}, argument())
}

//go:noinline
func observe(e *entry) {
	e.level++
	grow(24)
	runtime.GC()
}

//go:noinline
func sendObservable(c checker, level byte, a, b *node) uint64 {
	e := entry{level: level, first: a, last: b}
	observe(&e)
	return c.check(e, 0)
}

func nilReceiver() {
	defer func() {
		if recover() == nil {
			panic("nil receiver did not panic")
		}
	}()
	sendWithEffects(nil, 7, &node{11}, &node{19})
}

func main() {
	var c checker = sink{}
	for i := 0; i < 256; i++ {
		a, b := &node{uint64(i + 1000)}, &node{uint64(i + 2000)}
		want := uint64(byte(i)) + a.value + b.value
		if got := send(c, byte(i), a, b); got != want {
			panic("constructed argument corrupted across GC or stack growth")
		}
		if got := sendWithEffects(c, byte(i), a, b); got != want+uint64(effects) {
			panic("argument evaluation changed the snapshot")
		}
		want = uint64(byte(i+1)) + a.value + b.value
		if got := sendObservable(c, byte(i), a, b); got != want {
			panic("address-observable initialization moved")
		}
	}
	before := effects
	nilReceiver()
	if effects != before+1 {
		panic("nil check moved before argument effects")
	}
}
