// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

func undefined()

// Keep references in their original functions so diagnostics test the
// linker's per-symbol deduplication independently of backend inlining.
//
//go:noinline
func defined1() int {
	// To check multiple errors for a single symbol,
	// reference undefined more than once.
	undefined()
	undefined()
	return 0
}

//go:noinline
func defined2() {
	undefined()
	undefined()
}

func init() {
	_ = defined1()
	defined2()
}

// The "main" function remains undeclared.
