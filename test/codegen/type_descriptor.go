// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-DAG: %go.runtime.StructType = type <{
// LLVM-DAG: %go.runtime.UncommonType = type <{
// LLVM-DAG: %go.runtime.StructField = type <{
// LLVM-DAG: @"type:codegen.llvmTypeDescriptor" = constant <{ %go.runtime.StructType, %go.runtime.UncommonType, [2 x %go.runtime.StructField] }>
// LLVM-DAG: @"type:*codegen.llvmTypeDescriptor" = constant <{ %go.runtime.PtrType }>{{.*}}!goobj.symbol.flags
// LLVM-DAG: !goobj.relocs
// LLVM-DAG: !{i32 {{[0-9]+}}, i32 5}

// llvmTypeDescriptor exercises the compiler-owned reflectdata path. LLVM data
// lowering must preserve the resulting Go type descriptors and relocation
// metadata rather than reconstructing their runtime layout.
type llvmTypeDescriptor struct {
	p *byte
	x int
}

func useLLVMTypeDescriptor(v llvmTypeDescriptor) int {
	return v.x
}
