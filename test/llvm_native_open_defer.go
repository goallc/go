// run -gcflags=-enablellvm

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"runtime"
	"time"
)

func main() {
	done := make(chan struct{})
	go exitAfterDefer(done)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		panic("open-coded defer was not run by runtime.Goexit")
	}
}

func exitAfterDefer(done chan<- struct{}) {
	defer func() { done <- struct{}{} }()
	runtime.Goexit()
}
