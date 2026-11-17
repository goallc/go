package main

/*
extern int goValue(int);
static int invoke(int n) { return goValue(n) + 5; }
*/
import "C"

import "runtime"

//export goValue
func goValue(n C.int) C.int {
	runtime.GC()
	return n + 7
}

func main() {
	if C.invoke(30) != 42 {
		panic("C to Go callback")
	}
	println("LTO CGO OK")
}
