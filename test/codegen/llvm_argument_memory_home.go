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
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmRegisterArgumentMemoryHome(%codegen.llvmArgumentStrings3 %x)
// LLVM: [[HOME:%.*]] = alloca %codegen.llvmArgumentStrings3, align 8
// LLVM: store %codegen.llvmArgumentStrings3 %x, ptr [[HOME]], align 8
// LLVM: load ptr, ptr [[HOME]], align 8
// LLVM-NOT: .arg
// LLVM: ret i64
//
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmRegisterArgumentMemoryHome(%codegen.llvmArgumentStrings3 %x)
// LLVM-OPT-NOT: alloca
// LLVM-OPT-NOT: load
// LLVM-OPT-NOT: store
// LLVM-OPT: extractvalue %codegen.llvmArgumentStrings3 %x, 0, 1
// LLVM-OPT: extractvalue %codegen.llvmArgumentStrings3 %x, 1, 1
// LLVM-OPT: extractvalue %codegen.llvmArgumentStrings3 %x, 2, 1
// LLVM-OPT: ret i64
func llvmRegisterArgumentMemoryHome(x llvmArgumentStrings3) int {
	return len(x.a) + len(x.b) + len(x.c)
}

// Non-trivial arrays are assigned wholly to the ABI stack. The typed
// preallocated carrier binds LocalAddr directly to that incoming home, without
// constructing and initializing another local copy.
//
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmStackArgumentMemoryHome(ptr preallocated([2 x { ptr, i64 }]) align 8 %x)
// LLVM-NOT: alloca
// LLVM-NOT: store
// LLVM: load { ptr, i64 }, ptr
// LLVM: load { ptr, i64 }, ptr
// LLVM: ret i64
//
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmStackArgumentMemoryHome(ptr preallocated([2 x { ptr, i64 }]) align 8{{.*}} %x)
// LLVM-OPT-NOT: alloca
// LLVM-OPT-NOT: store
// LLVM-OPT: load i64, ptr
// LLVM-OPT: load i64, ptr
// LLVM-OPT: ret i64
//
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmDirectRegisterArgument(i64 %x)
// LLVM-NOT: alloca
// LLVM: add i64 %x, 1
// LLVM: ret i64
func llvmStackArgumentMemoryHome(x llvmArgumentStringArray) int {
	return len(x[0]) + len(x[1])
}
