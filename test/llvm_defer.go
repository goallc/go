// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

var deferTrace int
var deferResultFinalized = make(chan struct{}, 1)

type deferResultObject struct {
	value int
}

func normalDefers() (result int) {
	defer func() {
		deferTrace = deferTrace*10 + 1
		result += 10
	}()
	defer func() {
		deferTrace = deferTrace*10 + 2
		result += 2
	}()
	result = 5
	return
}

func recoveredDefer() (result int) {
	defer func() {
		if recover() != nil {
			result = 42
		}
	}()
	panic("defer recovery")
}

//go:noinline
func recoverWithArguments(value int, result *int) {
	if recover() != nil {
		*result = value
	}
}

func wrapperRecoveredDefer() (result int) {
	// Passing arguments requires a generated defer wrapper. Its GoObj FuncID
	// must be FuncIDWrapper so gorecover ignores that synthetic frame.
	defer recoverWithArguments(64, &result)
	panic("wrapper defer recovery")
}

func heapDefers(count int) (result int) {
	for i := 0; i < count; i++ {
		defer func(value int) {
			result = result*10 + value
		}(i)
	}
	return
}

func pointerDefer() (result int) {
	pointer := new(int)
	*pointer = 73
	captured := pointer
	defer func() {
		runtime.GC()
		result = *captured
	}()
	pointer = nil
	runtime.GC()
	return
}

func namedPointerResultSurvivesPanic() (result *deferResultObject) {
	result = &deferResultObject{value: 91}
	runtime.SetFinalizer(result, func(*deferResultObject) {
		deferResultFinalized <- struct{}{}
	})
	defer func() {
		runtime.GC()
		runtime.GC()
		runtime.GC()
		select {
		case <-deferResultFinalized:
			panic("named result was finalized during panic unwinding")
		default:
		}
		recover()
	}()
	panic("named result liveness")
}

func main() {
	if got := normalDefers(); got != 17 || deferTrace != 21 {
		panic("normal defer order or named result is incorrect")
	}
	if recoveredDefer() != 42 {
		panic("panic recovery did not update named result")
	}
	if wrapperRecoveredDefer() != 64 {
		panic("defer wrapper blocked panic recovery")
	}
	if heapDefers(3) != 210 {
		panic("heap defer order is incorrect")
	}
	if pointerDefer() != 73 {
		panic("defer lost a captured pointer across GC")
	}
	result := namedPointerResultSurvivesPanic()
	if result == nil || result.value != 91 {
		panic("defer lost a named pointer result during panic unwinding")
	}
	runtime.KeepAlive(result)
}
