// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-OPT-LABEL: define goabiinternal {{.*}}i64 @codegen.fusedLoop(
// LLVM-OPT-NOT: br
// LLVM-OPT: ret i64 0

// LLVM-LABEL: define goabiinternal i64 @codegen.fusedLoop(
// LLVM: br i1
// LLVM-NOT: br i1
// LLVM: ret i64

// LLVM-LABEL: define goabiinternal i64 @codegen.observedStore(
// LLVM: store i64 %a,
// LLVM: load i64,
// LLVM: store i64 %b,
// LLVM: ret i64

// LLVM-LABEL: define goabiinternal void @codegen.callObservedStore(
// LLVM: store i64 %a,
// LLVM: call goabiinternal void @codegen.observeCleanup(
// LLVM: store i64 %b,
// LLVM: ret void

// LLVM-LABEL: define goabiinternal void @codegen.partialZero(
// LLVM: call void @llvm.memset{{.*}}i64 32
// LLVM: store i64 %v,
// LLVM: ret void

// LLVM-LABEL: define goabiinternal void @codegen.overwrittenArray(
// LLVM: icmp eq ptr %p, null
// LLVM-NOT: store i64 %a,
// LLVM-COUNT-4: store i64 %b,
// LLVM-NOT: store
// LLVM: ret void

// LLVM-LABEL: define goabiinternal void @codegen.fullyInitialized(
// LLVM-NOT: @llvm.memset
// LLVM: ret void

//go:noescape
func observeCleanup(*[4]uint64)

func overwrittenArray(p *[4]uint64, a, b uint64) {
	*p = [4]uint64{a, a, a, a}
	*p = [4]uint64{b, b, b, b}
}

// DSE must retain initialization of the fields not overwritten below.
func partialZero(p *[4]uint64, v uint64) {
	*p = [4]uint64{}
	p[0] = v
}

// A possibly aliasing read must still observe the first write.
func observedStore(p, q *[4]uint64, a, b uint64) uint64 {
	p[0] = a
	x := q[0]
	p[0] = b
	return x
}

// Logical calls are memory readers even before expandCalls.
func callObservedStore(p *[4]uint64, a, b uint64) {
	p[0] = a
	observeCleanup(p)
	p[0] = b
}

// phiopt replaces the inner branch's result with a boolean conversion.
// Late fusion removes its now-empty arms before LLVM emission.
func fusedLoop(n int) int {
	x := 0
	for i := 0; i < n; i++ {
		if x == 0 {
			x = 0
		} else {
			x = 1
		}
	}
	return x
}

// The aggregate temporary is live, but its zeroing is fully overwritten.
func fullyInitialized(p *[16]uint64, v uint64) {
	*p = [16]uint64{v, v, v, v, v, v, v, v, v, v, v, v, v, v, v, v}
}
