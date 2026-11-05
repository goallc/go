// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"runtime"
	"unsafe"
)

type node struct{ value uint64 }

// 320 bytes on the supported 64-bit targets. The data fill is independent of
// the pointer fields; the word also has a partially overlapping scalar store.
type packet struct {
	word  uint64
	first *node
	data  [296]byte
	last  *node
}

type checker interface {
	check(packet, byte, uint64, uint64, uint64) uint64
}
type sink struct{}

var moved int
var effects int

//go:noinline
func sumSpace(space *[1024]byte) byte {
	var sum byte
	for _, v := range space {
		sum += v
	}
	return sum
}

// Read the address in a separate call so two conversions of the same local
// cannot be folded into one value across stack growth.
//
//go:noinline
func stackAddress(p *byte) uintptr {
	return uintptr(unsafe.Pointer(p))
}

//go:noinline
func grow(n int) byte {
	var space [1024]byte
	space[0] = byte(n)
	if n != 0 {
		space[1] = grow(n - 1)
	}
	// The out-of-line read retains the full stack object across recursion.
	return sumSpace(&space)
}

//go:noinline
func (sink) check(p packet, fill byte, word, first, last uint64) uint64 {
	var marker byte
	before := stackAddress(&marker)
	grow(128)
	if before != stackAddress(&marker) {
		moved++
	}
	runtime.GC()
	if p.word != word || p.first.value != first || p.last.value != last {
		panic("byval scalar snapshot or pointer corrupted")
	}
	for _, v := range p.data {
		if v != fill {
			panic("partial byval fill corrupted")
		}
	}
	result := p.word ^ p.first.value ^ p.last.value ^ uint64(fill)
	// A byval callee may change its own copy. Reusing the source at another
	// call must still observe the original value.
	p.word = ^word
	p.first, p.last = nil, nil
	for i := range p.data {
		p.data[i] = ^fill
	}
	runtime.KeepAlive(&p)
	return result
}

//go:noinline
func send(c checker, word uint64, upper uint32, fill byte, a, b *node, want uint64) uint64 {
	var p packet
	p.word = word
	*(*uint32)(unsafe.Add(unsafe.Pointer(&p.word), 4)) = upper
	p.first = a
	for i := range p.data {
		p.data[i] = fill
	}
	p.last = b
	return c.check(p, fill, want, 1001, 2003)
}

//go:noinline
func sendLargeZero(c checker, word uint64, a, b *node) uint64 {
	return c.check(packet{word: word, first: a, last: b}, 0, word, 1001, 2003)
}

//go:noinline
func sendStackPointers() uint64 {
	a, b := node{1001}, node{2003}
	return (sink{}).check(packet{word: 91, first: &a, last: &b}, 0, 91, 1001, 2003)
}

//go:noinline
func sendTwice(c checker, fill byte, a, b *node) uint64 {
	p := packet{word: 91, first: a, last: b}
	for i := range p.data {
		p.data[i] = fill
	}
	x := c.check(p, fill, 91, 1001, 2003)
	y := c.check(p, fill, 91, 1001, 2003)
	if x != y {
		panic("byval callee changed the caller's snapshot")
	}
	return y
}

//go:noinline
func argument() uint64 {
	effects++
	grow(128)
	runtime.GC()
	return 91
}

//go:noinline
func sendWithEffects(c checker, a, b *node) uint64 {
	return c.check(packet{word: 91, first: a, last: b}, 0, argument(), 1001, 2003)
}

func nilReceiver() {
	defer func() {
		if recover() == nil {
			panic("nil interface did not panic")
		}
	}()
	sendWithEffects(nil, &node{1001}, &node{2003})
}

func main() {
	var c checker = sink{}
	var endian uint16 = 1
	little := *(*byte)(unsafe.Pointer(&endian)) == 1
	for i := 0; i < 64; i++ {
		word := uint64(i)*0x102030405060708 + 0x8877665544332211
		upper := uint32(i)*0x10203 + 0xabcdef01
		want := word&0xffffffff00000000 | uint64(upper)
		if little {
			want = word&0xffffffff | uint64(upper)<<32
		}
		fill := byte(i * 37)
		done := make(chan bool)
		go func() {
			if got := sendStackPointers(); got != 91^1001^2003 {
				panic("stack pointers in byval argument did not relocate")
			}
			got := send(c, word, upper, fill, &node{1001}, &node{2003}, want)
			if got != want^1001^2003^uint64(fill) {
				panic("byval return value mismatch")
			}
			if got := sendLargeZero(c, word, &node{1001}, &node{2003}); got != word^1001^2003 {
				panic("large zero-initialized byval value mismatch")
			}
			sendTwice(c, fill, &node{1001}, &node{2003})
			done <- true
		}()
		<-done
	}
	if moved == 0 {
		panic("test did not observe stack relocation")
	}
	before := effects
	nilReceiver()
	if effects != before+1 {
		panic("nil check moved before argument effects")
	}
	if got := sendWithEffects(c, &node{1001}, &node{2003}); got != 91^1001^2003 {
		panic("intervening call changed the snapshot")
	}
}
