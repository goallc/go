// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// callFrame enters largeFrame through its ABI0 wrapper. If LLVM inlines
// largeFrame into the wrapper, the wrapper must check the enlarged frame.
//
//go:noescape
func callFrame(x int, p *int) int

func largeFrame(x int, p *int) int {
	var buf [4096]byte
	return useFrame(&buf, x) + *p
}

//go:noinline
func useFrame(buf *[4096]byte, x int) int {
	buf[x] = byte(x)
	return int(buf[x]) + int(buf[0])
}

func main() {
	// Start with a small goroutine stack, and keep a pointer into it live
	// across the wrapper's stack growth.
	done := make(chan int)
	go func() {
		value := 11
		done <- callFrame(7, &value)
	}()
	if got := <-done; got != 18 {
		panic(got)
	}
}
