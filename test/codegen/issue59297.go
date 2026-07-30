// asmcheck

// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-LABEL: define goabiinternal void @codegen.f(i64 %x, i64 %y, ptr %p)
// LLVM: call goabiinternal void @codegen.h(i64 8, i64 %x)
// LLVM: load volatile i8, ptr %p, align 1, !goallc.nilcheck !{{[0-9]+}}
// LLVM: store i64 %y, ptr %p, align 4

func f(x, y int, p *int) {
	// amd64:`MOVQ AX, BX`
	h(8, x)
	*p = y
}

//go:noinline
func h(a, b int) {
}
