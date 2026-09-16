// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

type llvmEqualityInterface interface {
	Value() int
}

type llvmEqualityValue int

func (v llvmEqualityValue) Value() int {
	return int(v)
}

// LLVM-DAG: define goabiinternal i1 @codegen.equalLLVMEmpty(
// LLVM-DAG: call goabiinternal i1 @"runtime.efaceeq<builtin.{{[0-9]+}}>"(ptr
func equalLLVMEmpty(a, b any) bool {
	return a == b
}

// LLVM-DAG: define goabiinternal i1 @codegen.equalLLVMNonEmpty(
// LLVM-DAG: call goabiinternal i1 @"runtime.ifaceeq<builtin.{{[0-9]+}}>"(ptr
func equalLLVMNonEmpty(a, b llvmEqualityInterface) bool {
	return a == b
}
