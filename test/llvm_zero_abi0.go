// run -gcflags=-std

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "unsafe"

type zeroABI0 [0]byte

var address uintptr

//go:cgo_unsafe_args
func zeroResult() (r zeroABI0) {
	address = uintptr(unsafe.Pointer(&r))
	return
}

//go:cgo_unsafe_args
func zeroArgument(v zeroABI0) {
	address = uintptr(unsafe.Pointer(&v))
}

func main() {
	address = 0
	zeroResult()
	if address == 0 {
		panic("zero-sized ABI0 result did not have an address")
	}

	address = 0
	zeroArgument(zeroABI0{})
	if address == 0 {
		panic("zero-sized ABI0 argument did not have an address")
	}
}
