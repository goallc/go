// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"runtime"
	"unsafe"
)

type words struct{ a, b, c uintptr }
type pointers struct{ p, q *int }

//go:noinline
func boxWords(x words) any { return x }

//go:noinline
func boxPointers(x pointers) any { return x }

func main() {
	checkConditionalAliases()
	x := words{1, 2, 3}
	a := boxWords(x)
	x.a = 11
	b := boxWords(x)
	if a.(words).a != 1 || b.(words).a != 11 {
		panic("box storage aliases source")
	}
	n, m := 7, 9
	p := pointers{&n, &m}
	c := boxPointers(p)
	p.p = &m
	d := boxPointers(p)
	n = 17
	runtime.GC()
	if c.(pointers).p != &n || *c.(pointers).p != 17 || d.(pointers).p != &m {
		panic("pointer fields lost original alias")
	}
	c1, c2 := make(chan struct{}), make(chan struct{})
	if c1 == c2 {
		panic("channel headers alias")
	}
	m1, m2 := make(map[int]int), make(map[int]int)
	m1[1] = 2
	if m2[1] != 0 {
		panic("map headers alias")
	}
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	runtime.KeepAlive(c)
	runtime.KeepAlive(d)
}

// Interface boxing exposes these helpers through ordinary Go conversion;
// convT16/convT32 do not permit external linkname references in the native linker.
type interfaceWords struct{ typ, data unsafe.Pointer }

//go:noinline
func scalar16(n uint16) unsafe.Pointer {
	v := any(n)
	return (*interfaceWords)(unsafe.Pointer(&v)).data
}

//go:noinline
func scalar32(n uint32) unsafe.Pointer {
	v := any(n)
	return (*interfaceWords)(unsafe.Pointer(&v)).data
}

//go:linkname scalar64 runtime.convT64
func scalar64(uint64) unsafe.Pointer

//go:noinline
func sliceWithCapacity(n int) []uint64 { return make([]uint64, n, 4) }

//go:noinline
func copiedSlice(src []uint64) []uint64 { dst := make([]uint64, 4); copy(dst, src); return dst }

//go:noinline
func stringBox(s string) any { return s[:3] }

var backing [8]byte

//go:noinline
func sliceBox() any { return backing[:0] }

//go:noinline
func dynamicSliceBox(s []byte) any { return s }

//go:noinline
func hintedMap(n int) map[int]int { return make(map[int]int, n) }

func checkConditionalAliases() {
	a16, b16 := scalar16(256), scalar16(256)
	a32, b32 := scalar32(256), scalar32(256)
	a64, b64 := scalar64(256), scalar64(256)
	if a16 == b16 || a32 == b32 || a64 == b64 {
		panic("uncached scalar boxes alias")
	}
	if *(*uint16)(a16) != 256 || *(*uint32)(a32) != 256 || *(*uint64)(a64) != 256 {
		panic("scalar box contents")
	}
	if scalar16(255) != scalar16(255) || scalar32(255) != scalar32(255) || scalar64(255) != scalar64(255) {
		panic("scalar cache identity")
	}
	s1, s2 := sliceWithCapacity(0), sliceWithCapacity(2)
	s1 = append(s1, 11, 12)
	if s2[0] != 0 || s2[1] != 0 || cap(s1) != 4 {
		panic("slice allocations alias")
	}
	src := []uint64{3, 5, 7}
	dst := copiedSlice(src)
	src[0] = 13
	if dst[0] != 3 || dst[1] != 5 || dst[2] != 7 || dst[3] != 0 {
		panic("slice copy alias/zero tail")
	}
	boxed := stringBox("abcdef")
	if boxed.(string) != "abc" {
		panic("string box")
	}
	empty := sliceBox().([]byte)
	if len(empty) != 0 || cap(empty) != 8 || unsafe.SliceData(empty) != &backing[0] {
		panic("slice box backing")
	}
	backing[0] = 23
	if empty[:1][0] != 23 {
		panic("slice backing lost alias")
	}
	if dynamicSliceBox(nil).([]byte) != nil {
		panic("nil slice box")
	}
	m1, m2 := hintedMap(32), hintedMap(32)
	m1[1] = 31
	if m2[1] != 0 {
		panic("hinted map headers alias")
	}
	runtime.GC()
	if *(*uint64)(a64) != 256 || dst[1] != 5 {
		panic("allocation lost across GC")
	}
	runtime.KeepAlive(a16)
	runtime.KeepAlive(b16)
	runtime.KeepAlive(a32)
	runtime.KeepAlive(b32)
	runtime.KeepAlive(a64)
	runtime.KeepAlive(b64)
}
