// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// LLVM-OBJVIEW-LABEL: TEXT codegen.llvmCodegenInvoke(SB)
// LLVM-OBJVIEW: R_CALLIND
// LLVM-OBJVIEW-LABEL: TEXT codegen.llvmCodegenInvokeInterface(SB)
// LLVM-OBJVIEW: R_CALLIND
// LLVM-LINK-DAG: nosplit: main.llvmCodegenInvoke{{<[^>]+>}} +{{[0-9]+}} -> indirect
// LLVM-LINK-DAG: nosplit: main.llvmCodegenInvokeInterface{{<[^>]+>}} +{{[0-9]+}} -> indirect
//
//go:noinline
//go:nosplit
func llvmCodegenInvoke(fn func(*int), value *int) {
	fn(value)
}

type llvmCodegenCaller interface {
	call(*int)
}

type llvmCodegenTargetType struct{}

//go:noinline
func (llvmCodegenTargetType) call(value *int) {
	*value += 2
}

//go:noinline
//go:nosplit
func llvmCodegenInvokeInterface(target llvmCodegenCaller, value *int) {
	target.call(value)
}

//go:noinline
func llvmCodegenTarget(value *int) {
	*value++
}

func main() {
	var value int
	llvmCodegenInvoke(llvmCodegenTarget, &value)
	llvmCodegenInvokeInterface(llvmCodegenTargetType{}, &value)
	if value != 3 {
		panic("indirect calls failed")
	}
}
