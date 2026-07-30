// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

type addressedResult struct {
	first  int
	second int
}

//go:noinline
func fillAddressedResult(result *addressedResult, seed int) {
	result.first = seed
	result.second = seed + 1
}

//go:noinline
func namedStackResult(seed int) (result addressedResult) {
	fillAddressedResult(&result, seed)
	return
}

func main() {
	const seed = 17
	result := namedStackResult(seed)
	if result.first != seed || result.second != seed+1 {
		panic("named stack result mismatch")
	}
}
