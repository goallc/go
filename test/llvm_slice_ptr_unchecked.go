// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "unsafe"

//go:noinline
func sliceData(s []byte) *byte {
	return unsafe.SliceData(s)
}

//go:noinline
func zeroArrayData(s []byte) *[0]byte {
	return (*[0]byte)(s)
}

func main() {
	var nilSlice []byte
	array := [1]byte{7}
	nonNilSlice := array[:]
	if sliceData(nilSlice) != nil || zeroArrayData(nilSlice) != nil {
		println("bad nil")
		return
	}
	if p := sliceData(nonNilSlice); p != &array[0] || *p != 7 {
		println("bad data")
		return
	}
	println("ok")
}
