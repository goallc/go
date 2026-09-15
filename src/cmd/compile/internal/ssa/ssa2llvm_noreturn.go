// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import "cmd/compile/internal/types"

func llvmNoReturnCall(v *Value) bool {
	if v.Op != OpStaticCall && v.Op != OpStaticLECall {
		return false
	}
	aux := auxToCall(v.Aux)
	if aux == nil || aux.Fn == nil {
		return false
	}
	// These APIs terminate the goroutine or process. The inliner's
	// NeverReturns heuristic is not a substitute: a panic may be recovered
	// by the callee's own defer, allowing the call to return normally.
	switch aux.Fn.Name {
	case "runtime.Goexit", "os.Exit",
		"testing.(*common).Skip", "testing.(*common).Skipf", "testing.(*common).SkipNow",
		"testing.(*common).Fatal", "testing.(*common).Fatalf", "testing.(*common).FailNow":
		return true
	}
	return false
}

// llvmLowerNoReturnCalls runs only at the LLVM lowering boundary. In particular,
// if !feature { t.Skip() } must not leave a false path into a SIMD operation
// when LLVM infers caller preconditions. Leave the native Go SSA pipeline alone.
func llvmLowerNoReturnCalls(f *Func) {
	changed := false
	for _, b := range f.Blocks {
		found := false
		for _, v := range b.Values {
			found = found || llvmNoReturnCall(v)
		}
		if !found {
			continue
		}
		// Values are not scheduled yet. Use the same memory ordering as LLVM
		// emission so side effects before the first terminating call survive.
		sset := f.newSparseSet(f.NumValues())
		numbers := f.Cache.allocInt32Slice(f.NumValues())
		values := storeOrder(b.Values, sset, numbers)
		f.retSparseSet(sset)
		f.Cache.freeInt32Slice(numbers)
		for i, call := range values {
			if !llvmNoReturnCall(call) {
				continue
			}
			// Keep the unreachable tail in a detached block until deadcode
			// removes its values and uses, including side-effecting calls.
			tail := f.NewBlock(BlockExit)
			tail.Values = append(tail.Values, values[i+1:]...)
			for _, v := range tail.Values {
				v.Block = tail
			}
			b.Values = values[: i+1 : i+1]
			for len(b.Succs) != 0 {
				e := b.Succs[len(b.Succs)-1]
				e.b.removePred(e.i)
				for _, v := range e.b.Values {
					if v.Op == OpPhi {
						e.b.removePhiArg(v, e.i)
					}
				}
				b.removeSucc(len(b.Succs) - 1)
			}
			b.Reset(BlockExit)
			mem := call
			if !mem.Type.IsMemory() {
				mem = b.NewValue1I(call.Pos, OpSelectN, types.TypeMem, int64(call.Type.NumFields()-1), call)
			}
			b.SetControl(mem)
			changed = true
			break
		}
	}
	if changed {
		deadcode(f)
	}
}
