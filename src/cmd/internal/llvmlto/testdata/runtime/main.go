package main

import (
	"example.com/lto/dep"
	"reflect"
	"runtime"
	"strings"
)

func catch() (ok bool) { defer func() { ok = recover() == "lto-panic" }(); dep.Boom(); return }
func main() {
	ch := make(chan int, 4)
	for i := 1; i <= 4; i++ {
		go func(i int) { ch <- dep.Deep(256, dep.Make(i)) }(i)
	}
	total := 0
	for i := 0; i < 4; i++ {
		total += <-ch
	}
	if total != 2570 {
		panic("stack growth or goroutine roots")
	}
	if dep.Opaque(3) != 12 {
		panic("opaque")
	}
	dep.Assembly()
	if dep.AssemblyValue != 41 {
		panic("assembly")
	}
	if dep.Transitive(1) != 12 {
		panic("transitive")
	}
	if dep.Add(35) != 42 {
		panic("add")
	}
	for i := 0; i < 100; i++ {
		n := dep.Make(i)
		if dep.Collect(n) != 2*i+1 || dep.Read(n) != i {
			panic("gc")
		}
		if reflect.ValueOf(n).MethodByName("Get").Call(nil)[0].Int() != int64(i) {
			panic("reflect")
		}
	}
	if !catch() {
		panic("recover")
	}
	found := false
	for _, f := range dep.Frames() {
		if strings.Contains(f.Function, "dep.Frames") {
			found = true
		}
	}
	if !found {
		panic("frames")
	}
	runtime.GC()
	println("LTO RUNTIME OK")
}
