// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

var llvmInitTaskValue int
var llvmInitOnlySeed = 42

func init() {
	llvmInitTaskValue = llvmInitOnlySeed
}

func main() {
	if llvmInitTaskValue != 42 {
		panic("package init did not run")
	}
}
