// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

//go:noinline
func checkMap[K comparable](keys []K) {
	var absent map[K]int
	if absent[keys[0]] != 0 {
		panic("nil map lookup")
	}
	m := make(map[K]int, len(keys))
	for i, k := range keys {
		m[k] = i + 1
	}
	for i, k := range keys {
		if m[k] != i+1 {
			panic("map slot")
		}
		delete(m, k)
		if m[k] != 0 {
			panic("missing map slot")
		}
	}
}

//go:noinline
func mustPanic(f func()) {
	defer func() {
		if recover() == nil {
			panic("missing panic")
		}
	}()
	f()
}

//go:noinline
func makeChannel(n int) chan int { return make(chan int, n) }

//go:noinline
func equal(x, y any) bool { return x == y }

//go:noinline
func trySend(c chan int, v int) bool {
	select {
	case c <- v:
		return true
	default:
		return false
	}
}

type getter interface{ Get() int }
type value int

func (v value) Get() int { return int(v) }

//go:noinline
func assert(x any) getter { return x.(getter) }

func main() {
	checkMap([]uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	checkMap([]uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	checkMap([]string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"})
	checkMap([][2]int{{1, 2}, {3, 4}})
	a, b := 1, 2
	checkMap([]*int{&a, &b})
	// Indirect map elements and the fat lookup path must retain the zero value.
	large := make(map[int][256]byte)
	large[1] = [256]byte{7}
	if large[1][0] != 7 || large[2][0] != 0 {
		panic("large map value")
	}
	var nilmap map[int]int
	mustPanic(func() { nilmap[1] = 2 })
	badkey := any([]int{1})
	mustPanic(func() { _ = map[any]int{}[badkey] })
	mustPanic(func() { _ = equal(badkey, badkey) })
	if !equal(1, 1) || equal(1, 2) {
		panic("interface equality")
	}
	if assert(value(7)).Get() != 7 {
		panic("interface assertion")
	}
	mustPanic(func() { _ = assert(1) })
	c := makeChannel(1)
	if c == nil || !trySend(c, 42) || trySend(c, 43) {
		panic("nonblocking send")
	}
	if v, ok := <-c; v != 42 || !ok {
		panic("receive")
	}
	close(c)
	if v, ok := <-c; v != 0 || ok {
		panic("closed receive")
	}
	mustPanic(func() { trySend(c, 1) })
	mustPanic(func() { makeChannel(-1) })
	if makeChannel(0) == nil || trySend(nil, 1) {
		panic("zero/nil channel")
	}
	// Exercise a blocking receive and its synchronization, not just buffered IO.
	c = makeChannel(0)
	go func() { c <- 9 }()
	if v, ok := <-c; v != 9 || !ok {
		panic("blocking receive")
	}
}
