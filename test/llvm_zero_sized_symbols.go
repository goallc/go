// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

type Z struct{}

var boxed any = Z{}

//go:noinline
func aPrivateString() string {
	return "abc"
}

//go:noinline
func emptyString() string {
	var value string
	return value
}

func main() {
	if value := aPrivateString(); len(value) != 3 || value[0] != 'a' {
		panic("private string changed")
	}
	if value := emptyString(); len(value) != 0 {
		panic("non-empty zero string")
	}
	if _, ok := boxed.(Z); !ok {
		panic("zero-sized interface value lost")
	}
}
