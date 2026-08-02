// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "sync/atomic"

// LLVM-DAG: load atomic i32, ptr %p seq_cst, align 4
// LLVM-DAG: load atomic i64, ptr %p seq_cst, align 8
// LLVM-DAG: store atomic i32 %value, ptr %p seq_cst, align 4
// LLVM-DAG: cmpxchg ptr %p, i32 %old, i32 %new seq_cst seq_cst
// LLVM-DAG: extractvalue { i32, i1 }
// LLVM-DAG: zext i1
// LLVM-DAG: atomicrmw add ptr %p, i32 %delta seq_cst
//
//go:noinline
func llvmAtomicLoad32(p *uint32) uint32 {
	return atomic.LoadUint32(p)
}

//go:noinline
func llvmAtomicLoad64(p *uint64) uint64 {
	return atomic.LoadUint64(p)
}

//go:noinline
func llvmAtomicStore32(p *uint32, value uint32) {
	atomic.StoreUint32(p, value)
}

//go:noinline
func llvmAtomicCompareAndSwap32(p *uint32, old, new uint32) bool {
	return atomic.CompareAndSwapUint32(p, old, new)
}

//go:noinline
func llvmAtomicAdd32(p *uint32, delta uint32) uint32 {
	return atomic.AddUint32(p, delta)
}
