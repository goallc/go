// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

//go:noinline
//go:nosplit
func llvmInvoke(fn func(*int), value *int) {
	fn(value)
}

type llvmCaller interface {
	call(*int)
}

type llvmTargetType struct{}

//go:noinline
func (llvmTargetType) call(value *int) {
	*value += 2
}

//go:noinline
//go:nosplit
func llvmInvokeInterface(target llvmCaller, value *int) {
	target.call(value)
}

//go:noinline
func llvmTarget(value *int) {
	*value++
}

func main() {
	var value int
	llvmInvoke(llvmTarget, &value)
	llvmInvokeInterface(llvmTargetType{}, &value)
	if value != 3 {
		panic("indirect calls failed")
	}
}
