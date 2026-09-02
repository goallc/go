// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

type llvmDebugPair struct {
	value   int
	pointer *int
}

// LLVM-DAG: DICompileUnit(language: DW_LANG_Go
// LLVM-DAG: emissionKind: FullDebug
// LLVM-DAG: DICompositeType(tag: DW_TAG_structure_type, name: "codegen.llvmDebugPair"
// LLVM-DAG: DILocalVariable(name: "local"
// LLVM-DAG: #dbg_declare
// LLVM-DAG: !goobj.debug.funcs
// LLVM-DAG: inlinedAt:
// LLVM-OBJSUMMARY-DAG: LLVM symbol name={{".*"}} kind=SDWARFFCN
// LLVM-OBJSUMMARY-DAG: LLVM symbol name={{".*"}} kind=SDWARFABSFCN
// LLVM-OBJSUMMARY-DAG: LLVM symbol name={{".*"}} kind=SDWARFLINES
// LLVM-OBJSUMMARY-DAG: LLVM aux owner="codegen.llvmDebugEntry" type=dwarf_info
// LLVM-OBJSUMMARY-DAG: LLVM aux owner="codegen.llvmDebugEntry" type=dwarf_lines
// LLVM-OBJSUMMARY-DAG: LLVM relocation-count type=R_DWARFSECREF count={{[1-9][0-9]*}}
// LLVM-OBJSUMMARY-DAG: LLVM relocation-count type=R_USETYPE count={{[1-9][0-9]*}}
func llvmDebugInner(value int, pair *llvmDebugPair) int {
	local := value + pair.value
	llvmDebugObserve(&local)
	return local
}

func llvmDebugMiddle(value int, pair *llvmDebugPair) int {
	return llvmDebugInner(value, pair)
}

//go:noinline
func llvmDebugObserve(pointer *int) {
	if *pointer == -1 {
		panic("unreachable")
	}
}

//go:noinline
func llvmDebugEntry(value int, pair *llvmDebugPair) int {
	return llvmDebugMiddle(value, pair)
}
