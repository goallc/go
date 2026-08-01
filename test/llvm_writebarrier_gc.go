// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

type wbPayload struct {
	tag int
	pad [32]uintptr
}

type wbNode struct {
	payload *wbPayload
	next    *wbNode
}

type wbPair struct {
	left  *wbPayload
	right *wbNode
}

var wbRoot *wbNode

//go:noinline
func wbStore(dst **wbNode, value *wbNode) {
	*dst = value
}

//go:noinline
func wbMove(dst *wbPair, value wbPair) {
	*dst = value
}

//go:noinline
func wbZero(dst *wbPair) {
	*dst = wbPair{}
}

//go:noinline
func wbGrow(depth int, value *wbNode) *wbNode {
	var pad [4]uintptr
	pad[0] = uintptr(depth)
	if depth == 0 {
		runtime.GC()
		runtime.KeepAlive(pad)
		return value
	}
	result := wbGrow(depth-1, value)
	runtime.KeepAlive(pad)
	return result
}

func main() {
	runtime.GOMAXPROCS(2)
	started := make(chan struct{})
	gcDone := make(chan struct{})
	go func() {
		close(started)
		for i := 0; i < 64; i++ {
			garbage := make([]*[256]byte, 64)
			for j := 0; j < 64; j++ {
				garbage[j] = new([256]byte)
			}
			runtime.GC()
			runtime.KeepAlive(garbage)
		}
		close(gcDone)
	}()
	<-started

	pair := new(wbPair)
	for i := 1; i <= 8192; i++ {
		payload := &wbPayload{tag: i}
		node := &wbNode{payload: payload, next: wbRoot}
		wbStore(&wbRoot, node)
		wbMove(pair, wbPair{left: payload, right: node})
		if wbRoot == nil || wbRoot.payload == nil || wbRoot.payload.tag != i {
			panic("write-barrier store lost its pointer")
		}
		if pair.left == nil || pair.left.tag != i || pair.right != wbRoot {
			panic("write-barrier move lost its pointers")
		}
		wbZero(pair)
		if pair.left != nil || pair.right != nil {
			panic("write-barrier zero retained pointers")
		}
		if i%257 == 0 {
			if got := wbGrow(256, wbRoot); got == nil || got.payload.tag != i {
				panic("stack growth lost a write-barrier result")
			}
		}
	}
	<-gcDone
	runtime.GC()
	if wbRoot == nil || wbRoot.payload == nil || wbRoot.payload.tag != 8192 {
		panic("final write-barrier pointer is invalid")
	}
}
