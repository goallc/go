// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-LABEL: define goabiinternal void @codegen.overwrittenCopy(
// LLVM-NOT: alloca
// LLVM-NOT: @llvm.lifetime.start
// LLVM-NOT: store i64 %a,
// LLVM: icmp eq ptr %p, null
// LLVM: call goabiinternal void @runtime.panicmem()
// LLVM-COUNT-8: store i64 %b,
// LLVM: ret void

// LLVM-LABEL: define goabiinternal void @codegen.partialCopy(
// LLVM: alloca [8 x i64]
// LLVM-COUNT-8: store i64 %a,
// LLVM: call void @llvm.memmove.p0.p0.i64(
// LLVM: store i64 %b,
// LLVM: ret void

// DSE removes the copy to p. Its source then becomes an unread local.
func overwrittenCopy(p *[8]uint64, a, b uint64) {
	t := [8]uint64{a, a, a, a, a, a, a, a}
	*p = t
	p[0] = b
	p[1] = b
	p[2] = b
	p[3] = b
	p[4] = b
	p[5] = b
	p[6] = b
	p[7] = b
}

// A partial overwrite must retain the source and the copy.
func partialCopy(p *[8]uint64, a, b uint64) {
	t := [8]uint64{a, a, a, a, a, a, a, a}
	*p = t
	p[0] = b
}
