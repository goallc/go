// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

type config struct {
	first  *int
	count  int
	second *int
	tail   [2]*int
}

var sink uintptr

//go:noinline
func value(n int) *int { return &n }

//go:noinline
func sumPadding(p *[256]uintptr) uintptr {
	var n uintptr
	for _, x := range p {
		n += x
	}
	return n
}

// Keep real frames live across recursion so each new goroutine exercises
// stack copying as well as GC while the selected config remains live.
//
//go:noinline
func grow(depth int) {
	var pad [256]uintptr
	pad[depth&255] = uintptr(depth + 1)
	if depth == 0 {
		runtime.GC()
	} else {
		grow(depth - 1)
	}
	sink += sumPadding(&pad)
}

// The nil path creates a pointer-bearing local; the other path can select
// either a caller's stack object or a heap object. Reading different fields
// across several calls must keep the common base and its contents valid.
//
//go:noinline
func execute(c *config, branch, fail bool) int {
	if c == nil {
		c = &config{value(11), 13, value(17), [2]*int{value(19), value(23)}}
	}
	grow(96)
	n := *c.first + c.count
	if branch {
		grow(24)
		n += *c.tail[0]
	} else {
		runtime.GC()
		n += *c.tail[1]
	}
	grow(64)
	n += *c.second
	if fail {
		panic(n)
	}
	runtime.GC()
	return n + *c.tail[0] + *c.tail[1]
}

//go:noinline
func caught(c *config, branch bool) (n int) {
	defer func() { n = recover().(int) }()
	return execute(c, branch, true)
}

//go:noinline
func heapConfig() *config {
	return &config{value(29), 31, value(37), [2]*int{value(41), value(43)}}
}

func main() {
	for mode := 0; mode < 3; mode++ {
		for _, branch := range []bool{false, true} {
			done := make(chan bool, 1)
			go func() {
				var local = config{value(29), 31, value(37), [2]*int{value(41), value(43)}}
				var c *config
				a, b, x, y, z := 11, 13, 17, 19, 23
				if mode == 1 {
					c = &local
				} else if mode == 2 {
					c = heapConfig()
				}
				if mode != 0 {
					a, b, x, y, z = 29, 31, 37, 41, 43
				}
				want := a + b + x + z
				if branch {
					want = a + b + x + y
				}
				if got := execute(c, branch, false); got != want+y+z {
					panic("mixed frame/object base lost across calls")
				}
				if got := caught(c, branch); got != want {
					panic("mixed frame/object base lost on panic path")
				}
				done <- true
			}()
			<-done
		}
	}
	if sink == 0 {
		panic("stack growth helper did not run")
	}
}
