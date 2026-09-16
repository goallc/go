//go:build !asan

// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"runtime"
	"unsafe"
)

//go:linkname allocateAligned runtime.mallocgc
func allocateAligned(size uintptr, typ unsafe.Pointer, zero bool) unsafe.Pointer

type interfaceHeader struct{ typ, data unsafe.Pointer }

// Keep the observation separate from attributed allocation calls: this tests
// physical addresses rather than an optimizer folding its own promises.
//
//go:noinline
func checkAllocationAddress(p unsafe.Pointer, alignment uintptr) {
	if uintptr(p)&(alignment-1) != 0 {
		panic("allocation alignment")
	}
}

func main() {
	var pointerType any = (*byte)(nil)
	scanType := (*interfaceHeader)(unsafe.Pointer(&pointerType)).typ
	cases := []struct {
		size, alignment uintptr
		scan            bool
	}{
		{1, 1, false}, {2, 2, false}, {6, 2, false}, {12, 4, false}, {16, 16, false},
		{17, 8, false}, {25, 32, false}, {32, 32, false}, {80, 16, false}, {128, 128, false},
		{512, 512, false}, {1024, 1024, false}, {32760, 8192, false}, {32768, 8192, false}, {65536, 8192, false},
		{32, 32, true}, {128, 128, true}, {512, 512, true}, {520, 8, true}, {1024, 8, true},
		{32760, 8, true}, {32768, 8192, true},
	}
	var live []unsafe.Pointer
	for repeat := 0; repeat < 40; repeat++ {
		for _, tc := range cases {
			var typ unsafe.Pointer
			if tc.scan {
				typ = scanType
			}
			alignment := tc.alignment
			if tc.scan && tc.size == 512 && unsafe.Sizeof(uintptr(0)) == 4 {
				alignment = 8 // The 32-bit allocator needs a malloc header here.
			}
			p := allocateAligned(tc.size, typ, true)
			checkAllocationAddress(p, alignment)
			live = append(live, p)
		}
	}
	runtime.KeepAlive(live)
}
