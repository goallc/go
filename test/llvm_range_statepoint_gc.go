// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

// wide is deliberately too large for the native compiler's scaled-index
// range lowering. The LLVM lowering computes each element from the relocatable
// array base and the integer loop index instead.
type wide struct {
	value   uint64
	padding [6]uint64
}

var sink uintptr

//go:noinline
func grow(depth int) uintptr {
	var padding [128]uintptr
	slot := depth & (len(padding) - 1)
	padding[slot] = uintptr(depth)
	if depth == 0 {
		runtime.GC()
		return padding[slot]
	}
	return grow(depth-1) + padding[slot]&1
}

//go:noinline
func rangeAcrossStackGrowth() {
	var values [32]wide
	for i := range values {
		values[i].value = uint64(i + 1)
	}

	for i, value := range values[:] {
		want := uint64(i + 1)
		if i != 0 {
			want += 1000
		}
		if value.value != want {
			panic("wide range element lost across stack growth")
		}
		if i+1 < len(values) {
			values[i+1].value += 1000
		}
		sink += grow(96)
	}
}

func main() {
	rangeAcrossStackGrowth()
}
