// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

var llvmAllocationGlobal int

type llvmAllocationPointers struct {
	global, heap *int
}

// A freshly allocated object needs no barrier for its global pointer, and
// only the new-value barrier for its possibly-heap pointer. The flags must
// reach LLVM before optimization so it need not rediscover the zero proof.
// LLVM-LABEL: define goabiinternal ptr @codegen.llvmAllocationWriteBarrier(
// LLVM-NOT: @goallc.gc.write.record
// LLVM: call void @goallc.gc.write.record(ptr {{[^,]+}}, ptr {{[^,]+}}, i32 1)
// LLVM-NOT: @goallc.gc.write.record
// LLVM: ret ptr
// LLVM-OPT-LABEL: define goabiinternal {{.*}}ptr @codegen.llvmAllocationWriteBarrier(
// LLVM-OPT-NOT: @goallc.gc.write.record
// LLVM-OPT: call void @goallc.gc.write.record(ptr {{[^,]+}}, ptr {{[^,]+}}, i32 1)
// LLVM-OPT-NOT: @goallc.gc.write.record
// LLVM-OPT: ret ptr
func llvmAllocationWriteBarrier(p *int) *llvmAllocationPointers {
	return &llvmAllocationPointers{global: &llvmAllocationGlobal, heap: p}
}
