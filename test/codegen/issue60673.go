// asmcheck

// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-LABEL: define goabiinternal void @codegen.g(ptr %p)
// LLVM: icmp eq ptr %p, null
// LLVM: call goabiinternal void @runtime.panicmem()
// LLVM-NEXT: unreachable
// LLVM: load i32, ptr %p, align 4
// LLVM: call goabiinternal void @codegen.f(i32 {{%.*}})
// LLVM-LABEL: define goabiinternal void @codegen.f(i32 %x)

//go:noinline
func f(x int32) {
}

func g(p *int32) {
	// argument marshaling code should live at line 17, not line 15.
	x := *p
	// 386: `MOVL [A-Z]+, \(SP\)`
	f(x)
}
