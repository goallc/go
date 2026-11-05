// run

package main

import (
	"runtime"
	"unsafe"
)

type cell struct {
	value int
	link  *cell
}
type iterator struct {
	remaining int
	item      *cell
}

var sink int
var addressCalls int
var visits int

//go:noinline
func snapshotAddress(p *cell) uintptr {
	addressCalls++
	return uintptr(unsafe.Pointer(p))
}

//go:noinline
func grow(depth int) int {
	var frame [128]int
	for i := range frame {
		frame[i] = depth + i
	}
	if depth != 0 {
		sink = grow(depth - 1)
	}
	runtime.KeepAlive(&frame)
	return frame[depth&127]
}

//go:noinline
func next(it *iterator) bool {
	if it.remaining == 0 {
		return false
	}
	it.remaining--
	visits++
	if it.remaining%7 == 0 {
		grow(80)
		runtime.GC()
	}
	if it.item.value != 123 || it.item.link.value != 456 {
		panic("iterator root")
	}
	return true
}

//go:noinline
func drain(it *iterator, keep *cell) int {
	for next(it) {
	}
	return keep.value + keep.link.value
}

//go:noinline
func makeIterator(item *cell) *iterator {
	return &iterator{remaining: 31, item: item}
}

//go:noinline
func repeat(b *cell, item *cell) int {
	i := 0
	for ; i < b.value; i++ {
		it := makeIterator(item)
		for next(it) {
		}
	}
	return i
}

//go:noinline
func repeatFixture() {
	tail := cell{value: 456}
	item := cell{value: 123, link: &tail}
	bound := cell{value: 3}
	before := snapshotAddress(&bound)
	oldVisits := visits
	if repeat(&bound, &item) != 3 || visits-oldVisits != 93 {
		panic("outer home or iterator reinitialization")
	}
	if before == snapshotAddress(&bound) {
		panic("outer fixture did not move stack")
	}
}

//go:noinline
func advance(p *cell) *cell {
	grow(48)
	runtime.GC()
	return p.link
}

//go:noinline
func changed(p *cell) int {
	sum := 0
	for p != nil {
		sum += p.value
		p = advance(p)
	}
	return sum
}

//go:noinline
func replace(address **cell, p *cell) bool {
	grow(48)
	runtime.GC()
	*address = p.link
	return *address != nil
}

//go:noinline
func aliased(address **cell) int {
	total := 0
	for {
		p := *address
		total += p.value
		if !replace(address, p) {
			break
		}
	}
	return total
}

//go:noinline
func exercise() {
	// keep and tail may remain in a caller frame. Each fresh goroutine starts
	// on a small stack, while next grows it before subsequent home reloads.
	tail := cell{value: 456}
	keep := cell{value: 123, link: &tail}
	moved := false
	for n := 0; n < 8; n++ {
		before := snapshotAddress(&keep)
		it := &iterator{remaining: 31, item: &keep}
		if drain(it, &keep) != 579 {
			panic("passive root")
		}
		moved = moved || before != snapshotAddress(&keep)
	}
	if !moved {
		panic("fixture did not move stack")
	}
	if changed(&keep) != 579 {
		panic("changed root")
	}
	p := &keep
	if aliased(&p) != 579 || p != nil {
		panic("aliased root")
	}
}

func main() {
	for n := 0; n < 3; n++ {
		done := make(chan bool)
		go func() { exercise(); done <- true }()
		<-done
		go func() { repeatFixture(); done <- true }()
		<-done
	}
	if addressCalls < 6 {
		panic("address observations missing")
	}
}
