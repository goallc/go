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

// LLVM-DAG: define goabiinternal i8 @codegen.equalLLVMEmpty(
// LLVM-DAG: call goabiinternal i8 @runtime.efaceeq(ptr
// LLVM-DAG: declare !goobj.builtin !{{[0-9]+}} goabiinternal i8 @runtime.efaceeq(ptr, ptr, ptr)
func equalLLVMEmpty(a, b any) bool {
	return a == b
}

// LLVM-DAG: define goabiinternal i8 @codegen.equalLLVMNonEmpty(
// LLVM-DAG: call goabiinternal i8 @runtime.ifaceeq(ptr
// LLVM-DAG: declare !goobj.builtin !{{[0-9]+}} goabiinternal i8 @runtime.ifaceeq(ptr, ptr, ptr)
func equalLLVMNonEmpty(a, b llvmEqualityInterface) bool {
	return a == b
}
