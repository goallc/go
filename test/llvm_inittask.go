// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

var llvmInitTaskValue int
var llvmInitOnlySeed = 42

func init() {
	// The runtime task must have completed before the package task starts.
	runtime.Gosched()
	llvmInitTaskValue = llvmInitOnlySeed
}

func init() {
	llvmInitTaskValue++
}

func main() {
	if llvmInitTaskValue != 43 {
		panic("package initialization order is incorrect")
	}
}
