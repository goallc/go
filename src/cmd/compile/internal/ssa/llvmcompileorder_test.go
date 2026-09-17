// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import (
	"cmd/compile/internal/ir"
	"sync"
	"testing"
)

func TestLLVMCompilationOrder(t *testing.T) {
	defer SetLLVMCompileOrder(nil)
	// A second batch must start from zero, including when it is smaller.
	for _, count := range []int{32, 3} {
		funcs := make([]*ir.Func, count)
		for i := range funcs {
			funcs[i] = new(ir.Func)
		}
		SetLLVMCompileOrder(funcs)
		var got []int
		var wg sync.WaitGroup
		for i := count - 1; i >= 0; i-- {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				defer llvmCompileOrder.enter(funcs[i])()
				// This shared state models the LLVM module and its caches.
				// The ordering gate must also provide mutual exclusion.
				got = append(got, i)
			}(i)
		}
		wg.Wait()
		for i, index := range got {
			if index != i {
				t.Fatalf("lowering order %v, want increasing indices", got)
			}
		}
	}
}
