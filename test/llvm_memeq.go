// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

func memeq(a, b string) bool {
	return a == b
}

func main() {
	equal17a := "x12345678901234567"
	equal17b := "y12345678901234567"
	println(
		memeq("a", "a"),
		memeq("a", "b"),
		memeq("short", "longer"),
		memeq(equal17a[1:], equal17b[1:]),
		memeq(equal17a[1:], "12345678901234568"),
	)
}
