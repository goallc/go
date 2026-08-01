// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

type newprocPayload struct {
	value int
	link  *newprocPayload
}

// launch grows the caller stack before materializing an escaping funcval and
// passing it to runtime.newproc. The funcval is the only object passed to the
// runtime call; its captured payload is checked after another GC.
//
//go:noinline
func launch(payload *newprocPayload, depth int, done chan<- int) {
	var padding [64]uintptr
	padding[0] = uintptr(depth)
	if depth != 0 {
		launch(payload, depth-1, done)
		runtime.KeepAlive(padding)
		return
	}
	go func() {
		runtime.GC()
		done <- payload.value + payload.link.value
	}()
}

func main() {
	const rounds = 8
	done := make(chan int, rounds)
	for i := 0; i < rounds; i++ {
		payload := &newprocPayload{value: 100 + i, link: &newprocPayload{value: i}}
		launch(payload, 256, done)
		payload = nil
		runtime.GC()
	}
	for i := 0; i < rounds; i++ {
		if got := <-done; got < 100 || got >= 100+2*rounds || got%2 != 0 {
			panic("runtime.newproc lost its pointer-typed funcval across GC or stack growth")
		}
	}
}
