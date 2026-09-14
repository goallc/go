// asmcheck

// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// The inlined call's unread temporary is removed before LLVM emission.
// LLVM-LABEL: define goabiinternal void @codegen.init()
// LLVM-NOT: alloca
// LLVM: store i64 1, ptr @codegen.x, align 8
// LLVM-NOT: alloca
// LLVM: ret void
// LLVM-DAG: define goabiinternal void @codegen.f(ptr %p)
// LLVM-DAG: icmp eq ptr %p, null
// LLVM-DAG: call goabiinternal void @runtime.panicmem()
// LLVM-DAG: store i64 1, ptr %p, align 8
// LLVM-DAG: !{ptr @codegen.x, ptr @"type:int"}

var x = func() int {
	n := 0
	f(&n)
	return n
}()

func f(p *int) {
	*p = 1
}

var y = 1

// z can be static initialized.
//
// amd64:-"MOVQ"
var z = y
