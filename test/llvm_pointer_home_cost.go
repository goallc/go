// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"runtime"
	"unsafe"
)

type node struct {
	value int
	next  *node
}

var visits, snapshots, sink int

//go:noinline
func address(p unsafe.Pointer) uintptr {
	snapshots++
	return uintptr(p)
}

//go:noinline
func grow(depth int) int {
	var frame [128]int
	for i := range frame {
		frame[i] = depth + i
	}
	if depth > 0 {
		sink = grow(depth - 1)
	}
	runtime.KeepAlive(&frame)
	return frame[depth&127]
}

//go:noinline
func safepoint(i int) {
	if i%5 == 0 {
		grow(80)
		runtime.GC()
	}
}

//go:noinline
func touch(p *node, i int) int {
	safepoint(i)
	visits++
	if p.next.value != 303 && p.next.value != 404 {
		panic("moved link")
	}
	if p.value != 101 && p.value != 202 {
		panic("moved root")
	}
	return p.value + p.next.value + i
}

//go:noinline
func sweep(p, q *node, n, mode int) int {
	total := p.value + q.value
	for i := 0; i < n; i++ {
		switch mode {
		case 0:
			total += touch(p, i)
			total += touch(q, i)
		case 1:
			if i&1 == 0 {
				total += touch(p, i)
			} else {
				total += touch(q, i)
			}
		case 2:
			if i%3 == 0 {
				total += touch(p, i)
			}
		}
	}
	return total
}

//go:noinline
func carried(base *[32]uint64, n int) uint64 {
	p := &base[0]
	var sum uint64
	for i := 0; i < n; i++ {
		safepoint(i)
		sum += *p
		p = (*uint64)(unsafe.Add(unsafe.Pointer(p), unsafe.Sizeof(*p)))
	}
	runtime.KeepAlive(base)
	return sum
}

//go:noinline
func exercise(mode int) {
	tailP, tailQ := node{value: 303}, node{value: 404}
	p, q := node{value: 101, next: &tailP}, node{value: 202, next: &tailQ}
	oldVisits := visits
	before := address(unsafe.Pointer(&p))
	got := sweep(&p, &q, 24, mode)
	want, count := 303, 0
	for i := 0; i < 24; i++ {
		switch mode {
		case 0:
			want += 404 + i + 606 + i
			count += 2
		case 1:
			if i&1 == 0 {
				want += 404 + i
			} else {
				want += 606 + i
			}
			count++
		case 2:
			if i%3 == 0 {
				want += 404 + i
				count++
			}
		}
	}
	if got != want || visits-oldVisits != count {
		panic("call-path roots")
	}
	if before == address(unsafe.Pointer(&p)) {
		panic("sweep stack did not move")
	}
	if sweep(&p, &q, 0, mode) != 303 {
		panic("zero-iteration entry")
	}
}

//go:noinline
func exerciseCarrier() {
	var data [32]uint64
	var want uint64
	for i := range data {
		data[i] = uint64(i*i + 7)
		want += data[i]
	}
	before := address(unsafe.Pointer(&data))
	if carried(&data, len(data)) != want {
		panic("derived carrier relocation")
	}
	if before == address(unsafe.Pointer(&data)) {
		panic("carrier stack did not move")
	}
}

func main() {
	for repeat := 0; repeat < 3; repeat++ {
		for mode := 0; mode < 3; mode++ {
			done := make(chan bool)
			go func(mode int) { exercise(mode); done <- true }(mode)
			<-done
		}
		done := make(chan bool)
		go func() { exerciseCarrier(); done <- true }()
		<-done
	}
	if snapshots != 24 {
		panic("address observations missing")
	}
}
