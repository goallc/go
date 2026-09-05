// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import "cmd/compile/internal/ir"

// planLocalLifetimes separates fixed stack allocation from storage lifetime.
// LocalAddr retains a variable identity, a defining block, and a memory input
// after Go SSA optimization. Repeated addresses name the same object; VarDef
// does not start a new object and is deliberately not part of this plan.
func (lfc *LLVMFuncContext) planLocalLifetimes(ordered map[ID][]*Value) {
	f := lfc.F
	addresses := make(map[llvmLocalKey][]*Value)
	var keys []llvmLocalKey
	for _, b := range f.Blocks {
		for _, v := range ordered[b.ID] {
			if v.Op != OpLocalAddr || v.Uses == 0 {
				continue
			}
			name, key := llvmLocalName(v)
			if name.Class == ir.PPARAM || lfc.DeferResults[key] || lfc.OpenDeferSlots[key] != 0 {
				continue // initialized by the parameter/defer-home producer
			}
			if lfc.Locals[key].Value.IsAAllocaInst().IsNil() {
				continue // caller-owned ABI storage, not an owned local object
			}
			if len(addresses[key]) == 0 {
				keys = append(keys, key)
			}
			addresses[key] = append(addresses[key], v)
		}
	}
	lfc.LocalLifetimeValues = make(map[ID]bool)
	lfc.LocalLifetimeBlocks = make(map[ID][]llvmLocalKey)
	if len(keys) == 0 {
		return
	}
	idom, dom := f.Idom(), f.Sdom()
	cyclic := llvmLifetimeCyclicBlocks(f)
	for _, key := range keys {
		roots := addresses[key]
		block := roots[0].Block
		for _, root := range roots[1:] {
			for block != root.Block && !dom.isAncestor(block, root.Block) {
				block = idom[block.ID]
			}
		}
		var first *Value
		for _, v := range ordered[block.ID] {
			if v.Op == OpLocalAddr && v.Uses != 0 {
				_, candidate := llvmLocalName(v)
				if candidate == key {
					first = v
					break
				}
			}
		}
		// A start in a cycle executes repeatedly. Permit that only when a
		// complete write establishes this iteration's contents before any
		// intervening memory observation. Otherwise retain the contents across
		// iterations by starting in the nearest non-cyclic dominator. SCCs,
		// rather than just natural-loop headers, also cover irreducible CFGs.
		if cyclic[block.ID] && !(len(roots) == 1 && first != nil &&
			llvmLifetimeInitializedInBlock(first, ordered[block.ID])) {
			for cyclic[block.ID] {
				block = idom[block.ID]
			}
			first = nil
		}
		if first != nil {
			lfc.LocalLifetimeValues[first.ID] = true
		} else {
			lfc.LocalLifetimeBlocks[block.ID] = append(lfc.LocalLifetimeBlocks[block.ID], key)
		}
	}
}

// llvmLifetimeInitializedInBlock proves a deliberately small case, not a
// general memory liveness analysis. Unknown reads, calls, partial writes, and
// possibly aliasing moves require a lifetime outside the cycle. Pure SSA work
// and Go-only VarDef bookkeeping do not observe the storage.
func llvmLifetimeInitializedInBlock(address *Value, values []*Value) bool {
	seen := false
	for _, v := range values {
		if v == address {
			seen = true
			continue
		}
		if !seen || v.Op == OpVarDef || v.Op == OpPhi {
			continue
		}
		switch v.Op {
		case OpZero:
			return v.Args[0] == address && v.AuxInt == address.Type.Elem().Size()
		case OpStore:
			return v.Args[0] == address && v.Args[1].Type.Size() == address.Type.Elem().Size()
		}
		if v.Type.IsMemory() || v.MemoryArg() != nil {
			return false
		}
	}
	return false
}

// llvmLifetimeCyclicBlocks identifies every block which can execute more than
// once on a path. Only reachable blocks participate in LLVM emission.
func llvmLifetimeCyclicBlocks(f *Func) []bool {
	index, low := make([]int, f.NumBlocks()), make([]int, f.NumBlocks())
	onStack, cyclic := make([]bool, f.NumBlocks()), make([]bool, f.NumBlocks())
	var stack []*Block
	next := 0
	var visit func(*Block)
	visit = func(b *Block) {
		next++
		index[b.ID], low[b.ID] = next, next
		stack = append(stack, b)
		onStack[b.ID] = true
		for _, edge := range b.Succs {
			succ := edge.b
			if index[succ.ID] == 0 {
				visit(succ)
				low[b.ID] = min(low[b.ID], low[succ.ID])
			} else if onStack[succ.ID] {
				low[b.ID] = min(low[b.ID], index[succ.ID])
			}
		}
		if low[b.ID] != index[b.ID] {
			return
		}
		start := len(stack) - 1
		for stack[start] != b {
			start--
		}
		isCycle := len(stack)-start > 1
		for _, edge := range b.Succs {
			isCycle = isCycle || edge.b == b
		}
		for _, member := range stack[start:] {
			cyclic[member.ID], onStack[member.ID] = isCycle, false
		}
		stack = stack[:start]
	}
	visit(f.Entry)
	return cyclic
}
