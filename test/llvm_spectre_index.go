// run -gcflags=-spectre=index

//go:build amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

//go:noinline
func index(x []int, i int) int { return x[i] }

//go:noinline
func indexUnsigned(x []int, i uint) int { return x[i] }

//go:noinline
func indexArray(x *[4]int, i int) int { return x[i] }

//go:noinline
func indexString(x string, i int) byte { return x[i] }

//go:noinline
func slice(x []int, i, j int) []int { return x[i:j] }

//go:noinline
func slice3(x []int, i, j, k int) []int { return x[i:j:k] }

//go:noinline
func sliceString(x string, i, j int) string { return x[i:j] }

func mustPanic(f func()) {
	defer func() {
		if recover() == nil {
			panic("missing bounds panic")
		}
	}()
	f()
}

func main() {
	x := [4]int{11, 22, 33, 44}
	for i, want := range x {
		if index(x[:], i) != want || indexUnsigned(x[:], uint(i)) != want || indexArray(&x, i) != want {
			panic("incorrect masked index")
		}
		if indexString("abcd", i) != "abcd"[i] {
			panic("incorrect masked string index")
		}
	}
	for i := 0; i <= len(x); i++ {
		for j := i; j <= len(x); j++ {
			s := slice(x[:2], i, j) // slicing may extend beyond len to cap
			if len(s) != j-i || cap(s) != len(x)-i || (len(s) != 0 && s[0] != x[i]) {
				panic("incorrect masked slice")
			}
			if sliceString("abcd", i, j) != "abcd"[i:j] {
				panic("incorrect masked string slice")
			}
			for k := j; k <= len(x); k++ {
				s = slice3(x[:2], i, j, k)
				if len(s) != j-i || cap(s) != k-i || (len(s) != 0 && s[0] != x[i]) {
					panic("incorrect masked full slice")
				}
			}
		}
	}
	for _, i := range []int{-1, len(x), int(^uint(0) >> 1)} {
		mustPanic(func() { index(x[:], i) })
		mustPanic(func() { indexUnsigned(x[:], uint(i)) })
		mustPanic(func() { indexArray(&x, i) })
		mustPanic(func() { indexString("abcd", i) })
	}
	mustPanic(func() { slice(x[:], -1, 0) })
	mustPanic(func() { slice(x[:], 0, 5) })
	mustPanic(func() { slice(x[:], 2, 1) })
	mustPanic(func() { slice3(x[:], 0, 2, 1) })
	mustPanic(func() { slice3(x[:], 0, 2, 5) })
	if slice(nil, 0, 0) != nil || slice3(nil, 0, 0, 0) != nil {
		panic("nil slice changed")
	}
}
