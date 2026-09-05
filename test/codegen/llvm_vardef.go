// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

type llvmVarDefResult struct {
	status   *int
	changes  [2]func()
	added    []any
	removed  []any
	assumed  *int
	inFlight []any
}

type llvmVarDefMap map[string]llvmVarDefResult

// llvmVarDefZero models a zero aggregate returned from a nil map lookup. SSA
// can remove the explicit return-slot zero because the slot was already zeroed
// on entry, leaving only OpVarDef between that initialization and the load.
// OpVarDef must not become llvm.lifetime.start: that would make the existing
// zero undefined and let O2 return stale register contents for status.
//
// LLVM-LABEL: define weak goabiinternal void @"codegen.(*llvmVarDefMap).llvmVarDefZero"(
// LLVM: call void @llvm.lifetime.start.p0(ptr
// LLVM-NOT: call void @llvm.lifetime.start
// LLVM-NOT: @llvm.fake.use
// LLVM: ret void
// LLVM-OPT-LABEL: define weak goabiinternal void @"codegen.(*llvmVarDefMap).llvmVarDefZero"(
// LLVM-OPT-NOT: undef
// LLVM-OPT-NOT: @llvm.fake.use
// LLVM-OPT: ret void
func (values llvmVarDefMap) llvmVarDefZero(name string) llvmVarDefResult {
	if values == nil {
		return llvmVarDefResult{}
	}
	return values[name]
}
