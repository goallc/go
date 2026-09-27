// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

func llvmTracebackLeaf() int

// A callee may inspect the calling goroutine's stack. Turning recursion into
// a loop loses those frames, which cannot be recovered from an inline tree.
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmTracebackRecursive(
// LLVM-OPT: call goabiinternal i64 @codegen.llvmTracebackRecursive(
// LLVM-OPT: }
//
//go:noinline
func llvmTracebackRecursive(n int) int {
	if n == 0 {
		return llvmTracebackLeaf()
	}
	return llvmTracebackRecursive(n - 1)
}
