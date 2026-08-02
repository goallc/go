// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

type llvmGenericPair[A, B any] struct {
	first  A
	second B
}

// Keep shape and concrete instantiations as separate named LLVM types. The
// wrapper must explicitly reshape the first-class aggregate returned by the
// shape body instead of erasing nominal type identity globally.
//
// LLVM-DAG: %"codegen.llvmGenericPair[int,string]" = type { i64, { ptr, i64 } }
// LLVM-DAG: %"codegen.llvmGenericPair[go.shape.int,go.shape.string]" = type { i64, { ptr, i64 } }
// LLVM-LABEL: define goabiinternal %"codegen.llvmGenericPair[int,string]" @"codegen.llvmMakeGenericPair[int,string]"(
// LLVM: [[SHAPE:%.*]] = call goabiinternal %"codegen.llvmGenericPair[go.shape.int,go.shape.string]" @"codegen.llvmMakeGenericPair[go.shape.int,go.shape.string]"
// LLVM: [[FIRST:%.*]] = extractvalue %"codegen.llvmGenericPair[go.shape.int,go.shape.string]" [[SHAPE]], 0
// LLVM: [[CONCRETE0:%.*]] = insertvalue %"codegen.llvmGenericPair[int,string]" undef, i64 [[FIRST]], 0
// LLVM: [[SECOND:%.*]] = extractvalue %"codegen.llvmGenericPair[go.shape.int,go.shape.string]" [[SHAPE]], 1
// LLVM: [[CONCRETE1:%.*]] = insertvalue %"codegen.llvmGenericPair[int,string]" [[CONCRETE0]], { ptr, i64 } [[SECOND]], 1
// LLVM: ret %"codegen.llvmGenericPair[int,string]" [[CONCRETE1]]
//
//go:noinline
func llvmMakeGenericPair[A, B any](first A, second B) llvmGenericPair[A, B] {
	return llvmGenericPair[A, B]{first: first, second: second}
}

func llvmUseGenericPair(first int, second string) llvmGenericPair[int, string] {
	return llvmMakeGenericPair(first, second)
}
