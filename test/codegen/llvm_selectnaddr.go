// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

type llvmAddressedCallResult [20]int

// LLVM-LABEL: define goabiinternal i64 @codegen.llvmReadAddressedCallResult(
// LLVM: [[HOME:%[^ ]+\.home]] = alloca [20 x i64], align 8
// LLVM: call goabiinternal void @codegen.llvmMakeAddressedCallResult(i64 %seed, ptr goret([20 x i64]) align 8 "goretindex"="0" [[HOME]])
// LLVM-NOT: store [20 x i64]
// LLVM: load i64, ptr
// LLVM: ret i64
//
//go:noinline
func llvmReadAddressedCallResult(seed int) int {
	result := llvmMakeAddressedCallResult(seed)
	return result[0] + result[len(result)-1]
}

//go:noinline
func llvmMakeAddressedCallResult(seed int) (result llvmAddressedCallResult) {
	for i := range result {
		result[i] = seed + i
	}
	return
}
