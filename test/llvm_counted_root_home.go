// run

package main

import (
	"runtime"
	"unsafe"
)

type cell struct {
	value int
	next  *cell
}
type encoder interface{ Put([]byte, uint64) }
type stressEncoder struct{}

var order encoder = stressEncoder{}
var sink, visits, addressCalls int
var captured []byte

type writer interface{ Write([]byte) bool }
type checkingWriter struct{ words int }

//go:noinline
func (w *checkingWriter) Write(buffer []byte) bool {
	for len(buffer) != 0 {
		value := uint64(buffer[0]) | uint64(buffer[1])<<8 | uint64(buffer[2])<<16 | uint64(buffer[3])<<24 |
			uint64(buffer[4])<<32 | uint64(buffer[5])<<40 | uint64(buffer[6])<<48 | uint64(buffer[7])<<56
		w.words++
		if value != uint64(w.words) {
			panic("derived address after stack relocation")
		}
		buffer = buffer[8:]
	}
	return false
}

//go:noinline
func wordAddress(p *[33]uint64) uintptr {
	addressCalls++
	return uintptr(unsafe.Pointer(p))
}

//go:noinline
func writeData(w writer, data []uint64) {
	buffer := make([]byte, 16*8)
	for first := 0; first < len(data); first += 16 {
		end := first + 16
		if end > len(data) {
			end = len(data)
			buffer = buffer[:8*(end-first)]
		}
		chunk := data[first:end]
		for i, value := range chunk {
			order.Put(buffer[8*i:], value)
		}
		if w.Write(buffer) {
			return
		}
	}
}

//go:noinline
func derivedExercise() {
	var words [33]uint64
	for i := range words {
		words[i] = uint64(i + 1)
	}
	w := &checkingWriter{}
	visits = 0
	before := wordAddress(&words)
	writeData(w, words[:])
	if w.words != len(words) || visits != len(words) {
		panic("derived address visit count")
	}
	if before == wordAddress(&words) {
		panic("input array stack movement not observed")
	}
}

//go:noinline
func address(p *cell) uintptr {
	addressCalls++
	return uintptr(unsafe.Pointer(p))
}

//go:noinline
func grow(depth int) int {
	var frame [128]int
	for i := range frame {
		frame[i] = depth + i
	}
	if depth == 0 {
		runtime.GC()
	} else {
		sink = grow(depth - 1)
	}
	runtime.KeepAlive(&frame)
	return frame[depth&127]
}

//go:noinline
func (stressEncoder) Put(dst []byte, value uint64) {
	visits++
	if visits%7 == 1 {
		grow(80)
	}
	for i := 0; i < 8; i++ {
		dst[i] = byte(value >> uint(8*i))
	}
	// Capturing the user buffer must not expose the private root home itself.
	captured = dst[:8]
}

//go:noinline
func finishChunk(keep *cell, buffer []byte) int {
	sink += int(buffer[0])
	return keep.value + keep.next.value
}

//go:noinline
func counted(keep *cell, values []uint64, stop int) (uint64, int) {
	var sum uint64
	retained := keep.value + keep.next.value
	buf := make([]byte, 16*8)
	for first := 0; first < len(values); first += 16 {
		last := first + 16
		if last > len(values) {
			last = len(values)
		}
		chunk := values[first:last]
		for i, value := range chunk {
			if first+i == stop {
				goto done
			}
			order.Put(buf[8*i:], value)
			b := buf[8*i:]
			decoded := uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
				uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
			if decoded != value {
				panic("indirect encoding or capture")
			}
			sum += decoded
		}
		retained = finishChunk(keep, buf)
	}
done:
	return sum, retained
}

//go:noinline
func replace(slot **cell) {
	grow(32)
	*slot = (*slot).next
}

//go:noinline
func aliasLoop(slot **cell, n int) int {
	total := 0
	for i := 0; i < n; i++ {
		p := *slot
		total += p.value
		replace(slot)
	}
	return total
}

//go:noinline
func closureLoop(read func() *cell, change func(), n int) int {
	total := 0
	for i := 0; i < n; i++ {
		p := read()
		change()
		total += p.value
	}
	return total
}

//go:noinline
func exercise() {
	tail := cell{value: 456}
	keep := cell{value: 123, next: &tail}
	// Exactly one growing call, with GC at the deepest frame, prevents a later
	// shallow GC from obscuring whether the caller's stack actually moved.
	visits = 0
	before := address(&keep)
	sum, retained := counted(&keep, []uint64{0x123456789abcdef0}, -1)
	if sum != 0x123456789abcdef0 || retained != 579 {
		panic("live roots")
	}
	if before == address(&keep) {
		panic("stack movement not observed")
	}
	for _, n := range []int{0, 1, 15, 16, 17, 31, 32, 33} {
		values := make([]uint64, n)
		for i := range values {
			values[i] = uint64(i + 1)
		}
		for _, stop := range []int{-1, 0, n / 2, n} {
			used := n
			if stop >= 0 && stop < n {
				used = stop
			}
			old := visits
			sum, retained = counted(&keep, values, stop)
			if sum != uint64(used*(used+1)/2) || retained != 579 || visits-old != used {
				panic("count, boundary, or early exit")
			}
		}
	}
	for i := 0; i < 3; i++ {
		last := &cell{value: i + 7}
		first := &cell{value: i + 3, next: last}
		p := first
		if aliasLoop(&p, 2) != 2*i+10 || p != nil {
			panic("alias replacement")
		}
		p = first
		read := func() *cell { return p }
		change := func() { grow(32); p = p.next }
		if closureLoop(read, change, 2) != 2*i+10 || p != nil {
			panic("captured replacement")
		}
	}
}

func main() {
	for i := 0; i < 3; i++ {
		done := make(chan bool)
		go func() { exercise(); done <- true }()
		<-done
		go func() { derivedExercise(); done <- true }()
		<-done
	}
	if addressCalls != 12 || len(captured) != 8 {
		panic("observations missing")
	}
	captured = nil
}
