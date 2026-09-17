// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// Defer recovery metadata survives SSA removal of unreachable statements.
// Its bits slot must be allocated even without any remaining LocalAddr use.
//
// LLVM-LABEL: define goabiinternal void @codegen.llvmUnreachableDefer(
// LLVM: alloca i8, align 1, !goallc.open_defer_bits
// LLVM: store volatile i8 0, ptr
func llvmUnreachableDefer() {
	for {
	}
	defer func() {}()
	defer func() {}()
}
