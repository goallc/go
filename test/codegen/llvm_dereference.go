// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

type llvmDereferenceAddressedResult struct {
	first  int
	second int
}

type llvmDereferenceLargeResult struct {
	values [32]int
}

// LLVM-LABEL: define goabiinternal void @codegen.llvmNamedLargeStackResult(
// LLVM-SAME: i64 %seed, ptr goret(%codegen.llvmDereferenceLargeResult) align 8 "goretindex"="0" [[LARGE_RESULT:%[^)]+]])
// LLVM: call goabiinternal void @codegen.llvmFillNamedLargeStackResult(ptr [[LARGE_RESULT]], i64 %seed)
// LLVM-NOT: memmove
// LLVM: ret void
//
// LLVM-LABEL: define goabiinternal %codegen.llvmDereferenceAddressedResult @codegen.llvmNamedStackResult(
// LLVM: call goabiinternal void @codegen.llvmFillNamedStackResult(
// LLVM: load %codegen.llvmDereferenceAddressedResult, ptr {{%.*}}, align 8
// LLVM: ret %codegen.llvmDereferenceAddressedResult
//
//go:noinline
func llvmNamedStackResult(seed int) (result llvmDereferenceAddressedResult) {
	llvmFillNamedStackResult(&result, seed)
	return
}

//go:noinline
func llvmFillNamedStackResult(result *llvmDereferenceAddressedResult, seed int) {
	result.first = seed
	result.second = seed + 1
}

//go:noinline
func llvmNamedLargeStackResult(seed int) (result llvmDereferenceLargeResult) {
	llvmFillNamedLargeStackResult(&result, seed)
	return
}

//go:noinline
func llvmFillNamedLargeStackResult(result *llvmDereferenceLargeResult, seed int) {
	for i := range result.values {
		result.values[i] = seed + i
	}
}
