// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

type llvmNilNode struct {
	value int
	next  *llvmNilNode
}

type llvmNilRoots struct {
	checked *llvmNilNode
	live    *llvmNilNode
}

//go:noinline
func llvmNilUseRoots(roots *llvmNilRoots) int {
	return roots.checked.value + roots.live.value + roots.live.next.value
}

//go:noinline
func llvmNilRead(checked, live *llvmNilNode) int {
	roots := llvmNilRoots{checked: checked, live: live}
	checkedValue := roots.checked.value
	runtime.GC()
	return checkedValue + llvmNilUseRoots(&roots)
}

func main() {
	tail := &llvmNilNode{value: 13}
	live := &llvmNilNode{value: 11, next: tail}
	checked := &llvmNilNode{value: 7}
	if got, want := llvmNilRead(checked, live), 7+7+11+13; got != want {
		panic("non-nil explicit nil check or live pointer roots failed")
	}
	defer func() {
		if recover() == nil {
			panic("nil explicit nil check did not produce a recoverable panic")
		}
		if live.value != 11 || live.next != tail || tail.value != 13 {
			panic("live pointers were corrupted after recovered nil check")
		}
	}()
	llvmNilRead(nil, live)
}
