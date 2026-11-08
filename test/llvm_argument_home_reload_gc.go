// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

//go:noinline
func sumHomeStack(p *[1024]int) int {
	sum := 0
	for _, v := range p {
		sum += v
	}
	return sum
}

//go:noinline
func growHomeStack(n int) int {
	var pad [1024]int
	pad[n%len(pad)] = n
	if n > 0 {
		growHomeStack(n - 1)
	} else {
		runtime.GC()
	}
	runtime.KeepAlive(&pad)
	return sumHomeStack(&pad)
}

//go:noinline
func inspectHome(dst []byte) int {
	return int(dst[7]) + int(dst[8]) + int(dst[10]) + int(dst[11])
}

//go:noinline
func writeHome(dst []byte, index int, branch bool) {
	if growHomeStack(40) != 40 {
		panic("stack frame corrupted")
	}
	dst[index] = 11
	dst[index+1] = 12
	if branch {
		dst[index+2] = 13
	} else {
		dst[index+2] = 14
	}
	if growHomeStack(80) != 80 {
		panic("stack frame corrupted")
	}
	dst[index+3] = 15
	dst[index+4] = 16
	// Preserve a whole-aggregate use after the calls as well as address uses,
	// so the late pass selects an argument home rather than scalar spills.
	if inspectHome(dst) != 54 {
		panic("aggregate home contents corrupted")
	}
}

//go:noinline
func checkHome(branch bool) {
	var dst [32]byte // Must remain on the moving goroutine stack.
	writeHome(dst[:], 7, branch)
	want := byte(14)
	if branch {
		want = 13
	}
	if dst[7] != 11 || dst[8] != 12 || dst[9] != want || dst[10] != 15 || dst[11] != 16 {
		panic("aggregate argument home lost across stack growth")
	}
}

func main() {
	for i := 0; i < 20; i++ {
		done := make(chan bool)
		go func() { checkHome(i%2 == 0); done <- true }()
		<-done
	}
}
