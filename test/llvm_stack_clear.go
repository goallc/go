// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

//go:noinline
func fill(b []byte, v byte) {
	for i := range b {
		b[i] = v
	}
}

//go:noinline
func check(b []byte, v byte) {
	for _, x := range b {
		if x != v {
			panic("stack clear boundary")
		}
	}
}

//go:noinline
func grow(n int) {
	var pad [1024]byte
	fill(pad[:], byte(n))
	if n == 0 {
		runtime.GC()
	} else {
		grow(n - 1)
	}
	check(pad[:], byte(n))
}

//go:noinline
func initializeRoots(a []*int) {
	for i := range a {
		if a[i] != nil {
			panic("uninitialized root")
		}
		if i%2 == 0 {
			a[i] = new(int)
			*a[i] = i + 1
		}
	}
}

//go:noinline
func checkRoots(a []*int) {
	for i, p := range a {
		if i%2 == 0 {
			if p == nil || *p != i+1 {
				panic("lost root")
			}
		} else if p != nil {
			panic("nonzero root")
		}
	}
}

//go:noinline
func roots() {
	var a [96]*int
	initializeRoots(a[:])
	grow(128)
	runtime.GC()
	checkRoots(a[:])
	a = [96]*int{}
	runtime.GC()
	for _, p := range a {
		if p != nil {
			panic("root clear")
		}
	}
	runtime.KeepAlive(&a)
}

//go:noinline
func largeRoots() {
	var a [512]*int
	initializeRoots(a[:])
	grow(128)
	runtime.GC()
	checkRoots(a[:])
	a = [512]*int{}
	runtime.GC()
	for _, p := range a {
		if p != nil {
			panic("large root clear")
		}
	}
	runtime.KeepAlive(&a)
}

//go:noinline
func clear0At1() {
	var a struct {
		before [1]byte
		data   [0]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [0]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear1At1() {
	var a struct {
		before [1]byte
		data   [1]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [1]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear15At1() {
	var a struct {
		before [1]byte
		data   [15]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [15]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear16At1() {
	var a struct {
		before [1]byte
		data   [16]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [16]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear255At1() {
	var a struct {
		before [1]byte
		data   [255]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [255]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear256At1() {
	var a struct {
		before [1]byte
		data   [256]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [256]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear257At1() {
	var a struct {
		before [1]byte
		data   [257]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [257]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear511At1() {
	var a struct {
		before [1]byte
		data   [511]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [511]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear512At1() {
	var a struct {
		before [1]byte
		data   [512]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [512]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear767At1() {
	var a struct {
		before [1]byte
		data   [767]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [767]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear768At1() {
	var a struct {
		before [1]byte
		data   [768]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [768]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear768At8() {
	var a struct {
		before [8]byte
		data   [768]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [768]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear768At15() {
	var a struct {
		before [15]byte
		data   [768]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [768]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear769At1() {
	var a struct {
		before [1]byte
		data   [769]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [769]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear769At8() {
	var a struct {
		before [8]byte
		data   [769]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [769]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear769At15() {
	var a struct {
		before [15]byte
		data   [769]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [769]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear1023At1() {
	var a struct {
		before [1]byte
		data   [1023]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [1023]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear1024At1() {
	var a struct {
		before [1]byte
		data   [1024]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [1024]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear1025At1() {
	var a struct {
		before [1]byte
		data   [1025]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [1025]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear4096At1() {
	var a struct {
		before [1]byte
		data   [4096]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [4096]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear191At1() {
	var a struct {
		before [1]byte
		data   [191]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [191]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear192At1() {
	var a struct {
		before [1]byte
		data   [192]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [192]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear193At1() {
	var a struct {
		before [1]byte
		data   [193]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [193]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear513At1() {
	var a struct {
		before [1]byte
		data   [513]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [513]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear4035At1() {
	var a struct {
		before [1]byte
		data   [4035]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [4035]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear4095At1() {
	var a struct {
		before [1]byte
		data   [4095]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [4095]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear4097At1() {
	var a struct {
		before [1]byte
		data   [4097]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [4097]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear4099At1() {
	var a struct {
		before [1]byte
		data   [4099]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [4099]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

//go:noinline
func clear65027At1() {
	var a struct {
		before [1]byte
		data   [65027]byte
		after  [17]byte
	}
	check(a.data[:], 0)
	fill(a.before[:], 0x53)
	fill(a.data[:], 0xab)
	fill(a.after[:], 0x97)
	a.data = [65027]byte{}
	check(a.before[:], 0x53)
	check(a.data[:], 0)
	check(a.after[:], 0x97)
}

func main() {
	var empty []byte
	clear(empty)
	clear(empty[:0])
	clear0At1()
	clear1At1()
	clear15At1()
	clear16At1()
	clear255At1()
	clear256At1()
	clear257At1()
	clear511At1()
	clear512At1()
	clear767At1()
	clear768At1()
	clear768At8()
	clear768At15()
	clear769At1()
	clear769At8()
	clear769At15()
	clear1023At1()
	clear1024At1()
	clear1025At1()
	clear4096At1()
	roots()
	largeRoots()
	clear191At1()
	clear192At1()
	clear193At1()
	clear513At1()
	clear4035At1()
	clear4095At1()
	clear4097At1()
	clear4099At1()
	clear65027At1()
}
