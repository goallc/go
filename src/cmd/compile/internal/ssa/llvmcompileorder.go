// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import (
	"cmd/compile/internal/ir"
	"sync"
)

// LLVM lowering shares a context, module, type caches, and debug builder.
// Serialize the whole lowering operation, in a fixed order so declarations
// and metadata do not depend on which SSA worker finishes first.
var llvmCompileOrder *llvmCompilationOrder

type llvmCompilationOrder struct {
	cond  sync.Cond
	index map[*ir.Func]int
	next  int
}

// SetLLVMCompileOrder sets the lowering order for one batch of functions.
// Call it only while backend workers are stopped, and clear it after they
// finish. The order must match a serial traversal of the scheduler, including
// closures: otherwise all workers could wait for a function still in the queue.
func SetLLVMCompileOrder(funcs []*ir.Func) {
	llvmCompileOrder = nil
	if len(funcs) == 0 {
		return
	}
	order := &llvmCompilationOrder{index: make(map[*ir.Func]int, len(funcs))}
	order.cond.L = new(sync.Mutex)
	for i, fn := range funcs {
		order.index[fn] = i
	}
	llvmCompileOrder = order
}

func (order *llvmCompilationOrder) enter(fn *ir.Func) func() {
	if order == nil {
		// Direct lowering callers outside the package compiler are serial.
		return func() {}
	}
	index, ok := order.index[fn]
	if !ok {
		panic("LLVM function missing from compilation order")
	}
	order.cond.L.Lock()
	for index != order.next {
		order.cond.Wait()
	}
	// The caller keeps its SSA cache until lowering returns. Waiting here
	// bounds live SSA to one function per worker and prevents cache reuse
	// while LLVM is still consuming the function.
	return func() {
		order.next++
		order.cond.Broadcast()
		order.cond.L.Unlock()
	}
}
