// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// Large, pure initializers are outlined. A reachable map must keep its
// initializer through R_KEEP even though the package-init call is weak.
var kept = map[int]int{
	0: 10, 1: 11, 2: 12, 3: 13, 4: 14, 5: 15,
	6: 16, 7: 17, 8: 18, 9: 19, 10: 20, 11: 21,
	12: 22, 13: 23, 14: 24, 15: 25, 16: 26, 17: 27,
	18: 28, 19: 29, 20: 30, 21: 31, 22: 32, 23: 33,
}

var calls int

//go:noinline
func effect() int {
	calls++
	return calls
}

// The map itself is unused, but its initializer has a side effect. It must
// not acquire the removable-call contract of the pure outlined initializer.
var effectful = map[int]int{
	0: effect(), 1: 11, 2: 12, 3: 13, 4: 14, 5: 15,
	6: 16, 7: 17, 8: 18, 9: 19, 10: 20, 11: 21,
	12: 22, 13: 23, 14: 24, 15: 25, 16: 26, 17: 27,
	18: 28, 19: 29, 20: 30, 21: 31, 22: 32, 23: 33,
}

func main() {
	if len(kept) != 24 || kept[0] != 10 || kept[23] != 33 {
		panic("reachable map initializer was removed")
	}
	if calls != 1 {
		panic("unused map initialization lost its side effect")
	}
}
