// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

// Exercise both sides of the small-copy lowering limits. Byte arrays also
// cover sources with no stronger alignment guarantee.
type byteArray interface {
	[64]byte | [127]byte | [128]byte | [129]byte | [135]byte | [136]byte |
		[137]byte | [255]byte | [256]byte | [257]byte | [264]byte | [512]byte
}

//go:noinline
func transform[T byteArray](value T) T {
	for i := 0; i < len(value); i++ {
		value[i] ^= byte(i*13 + 1)
	}
	return value
}

//go:noinline
func checkArray[T byteArray]() {
	var box struct {
		before [17]byte
		value  T
		after  [17]byte
	}
	for i := range box.before {
		box.before[i] = 0xa5
		box.after[i] = 0x5a
	}
	for i := 0; i < len(box.value); i++ {
		box.value[i] = byte(i*37 + 19)
	}
	original := box.value
	fn := transform[T]
	got := fn(box.value)
	for i := 0; i < len(got); i++ {
		if got[i] != original[i]^byte(i*13+1) || box.value[i] != original[i] {
			panic("small stack argument/result copy")
		}
	}
	for i := range box.before {
		if box.before[i] != 0xa5 || box.after[i] != 0x5a {
			panic("small stack copy overwrote neighboring data")
		}
	}
}

// On amd64 this has the same 136-byte size and 8-byte alignment as the
// aggregate that exposed the REP MOVS startup-cost regression.
type entry struct {
	pointer *int
	payload [128]byte
}

//go:noinline
func transformEntry(value entry) entry {
	runtime.GC()
	if *value.pointer != 73 {
		panic("small stack copy lost pointer")
	}
	value.payload[0] ^= 0x55
	value.payload[127] ^= 0xaa
	return value
}

//go:noinline
func callEntry(fn func(entry) entry, value entry) entry {
	return fn(value)
}

func main() {
	checkArray[[64]byte]()
	checkArray[[127]byte]()
	checkArray[[128]byte]()
	checkArray[[129]byte]()
	checkArray[[135]byte]()
	checkArray[[136]byte]()
	checkArray[[137]byte]()
	checkArray[[255]byte]()
	checkArray[[256]byte]()
	checkArray[[257]byte]()
	checkArray[[264]byte]()
	checkArray[[512]byte]()

	var box struct {
		before uint64
		value  entry
		after  uint64
	}
	box.before, box.after = 0x123456789abcdef0, 0xfedcba9876543210
	box.value.pointer = new(int)
	*box.value.pointer = 73
	for i := range box.value.payload {
		box.value.payload[i] = byte(i*37 + 19)
	}
	original := box.value
	want := original
	want.payload[0] ^= 0x55
	want.payload[127] ^= 0xaa
	if got := callEntry(transformEntry, box.value); got != want || box.value != original {
		panic("small pointer-containing stack copy")
	}
	if box.before != 0x123456789abcdef0 || box.after != 0xfedcba9876543210 {
		panic("aligned small stack copy overwrote neighboring data")
	}
}
