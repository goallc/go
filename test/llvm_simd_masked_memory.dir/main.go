// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"simd/archsimd"
	"syscall"
	"unsafe"
)

func checkMemory(elementBytes, lanes int, store func(unsafe.Pointer, uint64) bool) {
	page := syscall.Getpagesize()
	memory, err := syscall.Mmap(-1, 0, 2*page, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		panic(err)
	}
	defer syscall.Munmap(memory)
	if err := syscall.Mprotect(memory[page:], syscall.PROT_NONE); err != nil {
		panic(err)
	}
	for _, valid := range []int{0, 1, lanes / 2, lanes - 1, lanes} {
		active := ^uint64(0) >> (64 - valid)
		for _, mask := range []uint64{0, active, active & 0xaaaaaaaaaaaaaaaa, active & 1} {
			for i := range memory[:page] {
				memory[i] = 0xa5
			}
			start := page - valid*elementBytes
			p := unsafe.Add(unsafe.Pointer(&memory[0]), start)
			enabled := store(p, mask)
			for i, got := range memory[:page] {
				want := byte(0xa5)
				if enabled && i >= start {
					lane := (i - start) / elementBytes
					if mask>>lane&1 != 0 {
						want = byte(uint64(lane+1) >> (8 * ((i - start) % elementBytes)))
					}
				}
				if got != want {
					panic(fmt.Sprintf("masked store bytes=%d lanes=%d valid=%d mask=%#x byte=%d got=%#x want=%#x", elementBytes, lanes, valid, mask, i, got, want))
				}
			}
		}
	}
	// No lane is accessed, including when the pointer itself is nil.
	store(nil, 0)
}

func main() {
	checkMemory(4, 4, store32)
	checkMemory(8, 4, store64)
	checkMemory(1, 64, store8)
	checkMemory(2, 32, store16)
}

//go:noinline
func store32(p unsafe.Pointer, bits uint64) bool {
	if archsimd.X86.AVX2() {
		var values, masks [4]int32
		for i := range values {
			values[i] = int32(i + 1)
			if bits>>i&1 != 0 {
				masks[i] = -1
			}
		}
		mask := archsimd.LoadInt32x4(masks[:]).Less(archsimd.Int32x4{})
		archsimd.LoadInt32x4(values[:]).StoreArrayMasked((*[4]int32)(p), mask)
		return true
	}
	return false
}

//go:noinline
func store64(p unsafe.Pointer, bits uint64) bool {
	if archsimd.X86.AVX2() {
		var values, masks [4]int64
		for i := range values {
			values[i] = int64(i + 1)
			if bits>>i&1 != 0 {
				masks[i] = -1
			}
		}
		mask := archsimd.LoadInt64x4(masks[:]).Less(archsimd.Int64x4{})
		archsimd.LoadInt64x4(values[:]).StoreArrayMasked((*[4]int64)(p), mask)
		return true
	}
	return false
}

//go:noinline
func store8(p unsafe.Pointer, bits uint64) bool {
	if archsimd.X86.AVX512() {
		var values, masks [64]int8
		for i := range values {
			values[i] = int8(i + 1)
			if bits>>i&1 != 0 {
				masks[i] = -1
			}
		}
		mask := archsimd.LoadInt8x64(masks[:]).Less(archsimd.Int8x64{})
		archsimd.LoadInt8x64(values[:]).StoreArrayMasked((*[64]int8)(p), mask)
		return true
	}
	return false
}

//go:noinline
func store16(p unsafe.Pointer, bits uint64) bool {
	if archsimd.X86.AVX512() {
		var values, masks [32]int16
		for i := range values {
			values[i] = int16(i + 1)
			if bits>>i&1 != 0 {
				masks[i] = -1
			}
		}
		mask := archsimd.LoadInt16x32(masks[:]).Less(archsimd.Int16x32{})
		archsimd.LoadInt16x32(values[:]).StoreArrayMasked((*[32]int16)(p), mask)
		return true
	}
	return false
}
