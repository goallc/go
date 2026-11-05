// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// Both many pointer fields and a large pointer-free field inside a pointer
// aggregate must avoid SelectionDAG's scalar result/operand limit. Keep the
// runtime call after optimization so SROA cannot split pointer words either.

type llvmLargeCopyRecord struct {
	bytes [1 << 20]byte
	flag  bool
	ptr   *int
}

// LLVM-LABEL: define goabiinternal void @codegen.llvmLargeCopyRecordAssign(
// LLVM-NOT: load %codegen.llvmLargeCopyRecord,
// LLVM: call goabiinternal void @runtime.wbMove(
// LLVM: call goabiinternal void @"runtime.memmove<builtin.{{[0-9]+}}>"(ptr %dst, ptr %src, i64 1048592)
// LLVM: ret void
// LLVM-OPT-LABEL: define goabiinternal void @codegen.llvmLargeCopyRecordAssign(
// LLVM-OPT: call goabiinternal void @"runtime.memmove<builtin.{{[0-9]+}}>"(
// LLVM-OPT: ret void
// LLVM-LABEL: define goabiinternal void @codegen.llvmLargeCopyArray(
// LLVM-NOT: load [32769 x ptr],
// LLVM: call goabiinternal void @"runtime.memmove<builtin.{{[0-9]+}}>"(ptr %.result0, ptr %src, i64 262152)
// LLVM: ret void
// LLVM-OPT-LABEL: define goabiinternal void @codegen.llvmLargeCopyArray(
// LLVM-OPT: call goabiinternal void @"runtime.memmove<builtin.{{[0-9]+}}>"(
// LLVM-OPT: ret void

func llvmLargeCopyArray(src *[32769]*int) [32769]*int {
	return *src
}

func llvmLargeCopyRecordAssign(dst, src *llvmLargeCopyRecord) {
	*dst = *src
}
