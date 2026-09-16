// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "sync/atomic"

// LLVM-DAG: load atomic i32, ptr %p seq_cst, align 4
// LLVM-DAG: load atomic i64, ptr %p seq_cst, align 8
// LLVM-DAG: store atomic i32 %value, ptr %p seq_cst, align 4
// LLVM-DAG: store atomic i64 %value, ptr %p seq_cst, align 8
// LLVM-DAG: cmpxchg ptr %p, i32 %old, i32 %new seq_cst seq_cst
// LLVM-DAG: cmpxchg ptr %p, i64 %old, i64 %new seq_cst seq_cst
// LLVM-DAG: [[CAS32:%[^ ]+]] = extractvalue { i32, i1 } {{%[^,]+}}, 1
// LLVM-DAG: ret i1 [[CAS32]]
// LLVM-DAG: atomicrmw add ptr %p, i32 %delta seq_cst
// LLVM-DAG: atomicrmw add ptr %p, i64 %delta seq_cst
// LLVM-DAG: atomicrmw xchg ptr %p, i32 %value seq_cst
// LLVM-DAG: atomicrmw or ptr %p, i64 %mask seq_cst
// LLVM-DAG: atomicrmw and ptr %p, i64 %mask seq_cst
// LLVM-DAG: atomicrmw and ptr %p, i32 %mask seq_cst
// LLVM-DAG: atomicrmw or ptr %p, i32 %mask seq_cst
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
func llvmAtomicStore64(p *uint64, value uint64) {
	atomic.StoreUint64(p, value)
}

//go:noinline
func llvmAtomicCompareAndSwap32(p *uint32, old, new uint32) bool {
	return atomic.CompareAndSwapUint32(p, old, new)
}

//go:noinline
func llvmAtomicCompareAndSwap64(p *uint64, old, new uint64) bool {
	return atomic.CompareAndSwapUint64(p, old, new)
}

//go:noinline
func llvmAtomicAdd32(p *uint32, delta uint32) uint32 {
	return atomic.AddUint32(p, delta)
}

//go:noinline
func llvmAtomicAdd64(p *uint64, delta uint64) uint64 {
	return atomic.AddUint64(p, delta)
}

//go:noinline
func llvmAtomicExchange32(p *uint32, value uint32) uint32 {
	return atomic.SwapUint32(p, value)
}

//go:noinline
func llvmAtomicOr64(p *uint64, mask uint64) uint64 {
	return atomic.OrUint64(p, mask)
}

//go:noinline
func llvmAtomicAnd64(p *uint64, mask uint64) uint64 {
	return atomic.AndUint64(p, mask)
}

//go:noinline
func llvmAtomicAnd32(p *uint32, mask uint32) uint32 {
	return atomic.AndUint32(p, mask)
}

//go:noinline
func llvmAtomicOr32(p *uint32, mask uint32) uint32 {
	return atomic.OrUint32(p, mask)
}

// A nil atomic access must remain a faulting instruction after optimization;
// the runtime turns the fault into a panic that Go code can recover.
//
// LLVM-DAG: store atomic i32 0, ptr null seq_cst, align 4
// LLVM-OPT-DAG: store atomic i32 0, ptr null seq_cst, align 4
// LLVM-DAG: attributes #{{[0-9]+}} = { {{.*}}null_pointer_is_valid{{.*}} }
// LLVM-OPT-DAG: attributes #{{[0-9]+}} = { {{.*}}null_pointer_is_valid{{.*}} }
//
//go:noinline
func llvmAtomicStoreNil() {
	atomic.StoreInt32(nil, 0)
}
