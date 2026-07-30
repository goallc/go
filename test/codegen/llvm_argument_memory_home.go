// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

type llvmArgumentStrings3 struct {
	a, b, c string
}

type llvmArgumentStringArray [2]string

// A register-assigned parameter that Go SSA can use directly remains an LLVM
// SSA value and does not acquire a memory home.
func llvmDirectRegisterArgument(x int) int {
	return x + 1
}

// llvmArgumentStrings3 fits wholly in the ABIInternal integer-register budget
// but is too large for Go SSA's aggregate-value limit. LLVM gives only this
// memory-backed parameter a complete local home instead of reconstructing its
// individual ABI register pieces from the aggregate formal parameter.
//
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmRegisterArgumentMemoryHome(%codegen.llvmArgumentStrings3 %0)
// LLVM: [[HOME:%.*]] = alloca %codegen.llvmArgumentStrings3, align 8
// LLVM: store %codegen.llvmArgumentStrings3 %0, ptr [[HOME]], align 8
// LLVM: load ptr, ptr [[HOME]], align 8
// LLVM-NOT: .arg
// LLVM: ret i64
//
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmRegisterArgumentMemoryHome(%codegen.llvmArgumentStrings3 %0)
// LLVM-OPT-NOT: alloca
// LLVM-OPT-NOT: load
// LLVM-OPT-NOT: store
// LLVM-OPT: extractvalue %codegen.llvmArgumentStrings3 %0, 0, 1
// LLVM-OPT: extractvalue %codegen.llvmArgumentStrings3 %0, 1, 1
// LLVM-OPT: extractvalue %codegen.llvmArgumentStrings3 %0, 2, 1
// LLVM-OPT: ret i64
func llvmRegisterArgumentMemoryHome(x llvmArgumentStrings3) int {
	return len(x.a) + len(x.b) + len(x.c)
}

// Non-trivial arrays are assigned wholly to the ABI stack. Their Go SSA
// LocalAddr uses the same local-home initialization instead of reading an
// uninitialized alloca.
//
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmStackArgumentMemoryHome([2 x { ptr, i64 }] %0)
// LLVM: [[STACK_HOME:%.*]] = alloca [2 x { ptr, i64 }], align 8
// LLVM: store [2 x { ptr, i64 }] %0, ptr [[STACK_HOME]], align 8
// LLVM: ret i64
//
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmStackArgumentMemoryHome([2 x { ptr, i64 }] %0)
// LLVM-OPT-NOT: alloca
// LLVM-OPT: extractvalue [2 x { ptr, i64 }] %0, 0
// LLVM-OPT: extractvalue [2 x { ptr, i64 }] %0, 1
// LLVM-OPT: ret i64
//
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmDirectRegisterArgument(i64 %x)
// LLVM-NOT: alloca
// LLVM: add i64 %x, 1
// LLVM: ret i64
func llvmStackArgumentMemoryHome(x llvmArgumentStringArray) int {
	return len(x[0]) + len(x[1])
}
