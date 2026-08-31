// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-DAG: @codegen.llvmNoPtrBSS = global <{ [8 x i8] }> zeroinitializer, section ".noptrbss"{{.*}}!dbg
// LLVM-DAG: @codegen.llvmNoPtrData = global <{ [8 x i8] }> <{ [8 x i8] c"*\00\00\00\00\00\00\00" }>, section ".noptrdata"{{.*}}!dbg
var llvmNoPtrBSS int
var llvmNoPtrData = 42

type llvmPointerData struct {
	p *int
}

// LLVM-DAG: @codegen.llvmPointerBSS = global <{ [8 x i8] }> zeroinitializer, section ".bss"
// LLVM-DAG: @codegen.llvmInitializedPointerData = global <{ ptr }> <{ ptr @codegen.llvmNoPtrData }>, section ".data"
var llvmPointerBSS llvmPointerData
var llvmInitializedPointerData = llvmPointerData{p: &llvmNoPtrData}

func llvmUseDataSections() (*int, int, int, *int) {
	return llvmPointerBSS.p, llvmNoPtrBSS, llvmNoPtrData, llvmInitializedPointerData.p
}

// LLVM: !goobj.gotype = !{
// LLVM-NOT: !goobj.debug.globals
