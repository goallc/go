// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

type llvmOpenDeferNamedResult struct {
	value int
}

// LLVM-LABEL: define goabiinternal i64 @codegen.llvmOpenDeferTwo(i64 %value)
// LLVM: [[SLOTS:%.*]] = alloca [2 x ptr], align 8, !goallc.open_defer_slots ![[SLOTS_MD:[0-9]+]]
// LLVM: [[SLOT0:%.*]] = getelementptr i8, ptr [[SLOTS]], i64 0
// LLVM: [[SLOT1:%.*]] = getelementptr i8, ptr [[SLOTS]], i64 8
// LLVM: [[BITS:%.*]] = alloca i8, {{.*}}!goallc.open_defer_bits
// LLVM: callbr void @llvm.go.defer.edge()
// LLVM-NEXT: to label %{{.*}} [label %[[RECOVERY:[A-Za-z0-9_.]+]]]
// LLVM: store volatile ptr {{.*}}, ptr [[SLOT0]]
// LLVM: store volatile ptr {{.*}}, ptr [[SLOT1]]
// LLVM: [[RECOVERY]]:
// LLVM-NEXT: call goabiinternal void @"runtime.deferreturn<builtin.{{[0-9]+}}>"(), !dbg !{{[0-9]+}}
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmOpenDeferTwo(i64 %value)
// LLVM-OPT: [[SLOTS_OPT:%.*]] = alloca [2 x ptr], align 8, !goallc.open_defer_slots ![[SLOTS_OPT_MD:[0-9]+]]
// LLVM-OPT: [[SLOT1_OPT:%.*]] = getelementptr {{.*}}i8, ptr [[SLOTS_OPT]], i64 8
// LLVM-OPT: [[BITS_OPT:%.*]] = alloca i8, {{.*}}!goallc.open_defer_bits
// LLVM-OPT: callbr void @llvm.go.defer.edge()
// LLVM-OPT-NEXT: to label %{{.*}} [label %[[RECOVERY_OPT:[A-Za-z0-9_.]+]]]
// LLVM-OPT: store volatile ptr {{.*}}, ptr [[SLOTS_OPT]]
// LLVM-OPT: store volatile ptr {{.*}}, ptr [[SLOT1_OPT]]
// LLVM-OPT: [[RECOVERY_OPT]]:
// LLVM-OPT: call goabiinternal void @"runtime.deferreturn<builtin.{{[0-9]+}}>"(), !dbg !{{[0-9]+}}

// LLVM-LABEL: define goabiinternal %codegen.llvmOpenDeferNamedResult @codegen.llvmOpenDeferNamed(
// LLVM: open.defer.recovery:
// LLVM-NEXT: call goabiinternal void @"runtime.deferreturn<builtin.{{[0-9]+}}>"(), !dbg !{{[0-9]+}}
// LLVM: load volatile %codegen.llvmOpenDeferNamedResult
// LLVM: ret %codegen.llvmOpenDeferNamedResult
// LLVM: ![[SLOTS_MD]] = !{i32 2}
// LLVM-OPT-LABEL: define goabiinternal %codegen.llvmOpenDeferNamedResult @codegen.llvmOpenDeferNamed(
// LLVM-OPT: common.ret:
// LLVM-OPT: load volatile %codegen.llvmOpenDeferNamedResult
// LLVM-OPT: ret %codegen.llvmOpenDeferNamedResult
// LLVM-OPT: open.defer.recovery:
// LLVM-OPT: call goabiinternal void @"runtime.deferreturn<builtin.{{[0-9]+}}>"(), !dbg !{{[0-9]+}}
// LLVM-OPT: ![[SLOTS_OPT_MD]] = !{i32 2}

func llvmOpenDeferTwo(value int) (result int) {
	defer func() {
		result++
	}()
	defer func() {
		result += value
	}()
	return 3
}

func llvmOpenDeferNamed(value int) (result llvmOpenDeferNamedResult) {
	defer func() {
		result.value += value
	}()
	return llvmOpenDeferNamedResult{value: 3}
}
