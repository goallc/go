// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

var llvmDeferSink int

// LLVM-LABEL: define goabiinternal ptr @codegen.llvmDeferPointerResult(
// LLVM-SAME: ptr{{.*}} %pointer){{.*}} #[[LLVM_NOINLINE:[0-9]+]] gc "goallc"
// LLVM: [[POINTER_SLOTS:%.*]] = alloca [1 x ptr], align 8, !goallc.open_defer_slots
// LLVM: [[POINTER_SLOT0:%.*]] = getelementptr i8, ptr [[POINTER_SLOTS]], i64 0
// LLVM: [[POINTER_BITS:%.*]] = alloca i8, {{.*}}!goallc.open_defer_bits
// LLVM: [[RESULT:%.*]] = alloca ptr, {{.*}}!goallc.defer_result
// LLVM: callbr void @llvm.go.defer.edge()
// LLVM-NEXT: to label %{{.*}} [label %[[RECOVER:[A-Za-z0-9_.]+]]]
// LLVM: store volatile ptr {{.*}}, ptr [[POINTER_SLOT0]]
// LLVM: store volatile i8 1, ptr [[POINTER_BITS]]
// LLVM: [[RECOVER]]:
// LLVM-NEXT: call goabiinternal void @"runtime.deferreturn<builtin.{{[0-9]+}}>"()
// LLVM-NEXT: {{.*}} = load volatile ptr, ptr [[RESULT]]
// LLVM-OPT-LABEL: define goabiinternal ptr @codegen.llvmDeferPointerResult(
// LLVM-OPT-SAME: ptr{{.*}} %pointer){{.*}} #[[LLVM_NOINLINE:[0-9]+]] gc "goallc"
// LLVM-OPT: [[POINTER_SLOTS_OPT:%.*]] = alloca [1 x ptr], align 8, !goallc.open_defer_slots
// LLVM-OPT: [[POINTER_BITS_OPT:%.*]] = alloca i8, {{.*}}!goallc.open_defer_bits
// LLVM-OPT: [[RESULT_OPT:%.*]] = alloca ptr, {{.*}}!goallc.defer_result
// LLVM-OPT: callbr void @llvm.go.defer.edge()
// LLVM-OPT-NEXT: to label %{{.*}} [label %[[RECOVER_OPT:[A-Za-z0-9_.]+]]]
// LLVM-OPT: store volatile ptr {{.*}}, ptr [[POINTER_SLOTS_OPT]]
// LLVM-OPT: store volatile i8 1, ptr [[POINTER_BITS_OPT]]
// LLVM-OPT: [[RECOVER_OPT]]:
// LLVM-OPT-NEXT: call goabiinternal void @"runtime.deferreturn<builtin.{{[0-9]+}}>"()
// LLVM-OPT-NEXT: {{.*}} = load volatile ptr, ptr [[RESULT_OPT]]

// LLVM-LABEL: define goabiinternal i64 @codegen.llvmDeferStack(i64 %value)
// LLVM: [[STACK_SLOTS:%.*]] = alloca [1 x ptr], align 8, !goallc.open_defer_slots
// LLVM: [[STACK_SLOT0:%.*]] = getelementptr i8, ptr [[STACK_SLOTS]], i64 0
// LLVM: [[STACK_BITS:%.*]] = alloca i8, {{.*}}!goallc.open_defer_bits
// LLVM: [[STACK_RESULT:%.*]] = alloca i64
// LLVM: store volatile i64 0, ptr [[STACK_RESULT]]
// LLVM: callbr void @llvm.go.defer.edge()
// LLVM-NEXT: to label %[[STACK_NORMAL:.*]] [label %[[STACK_RECOVER:.*]]]
// LLVM-NOT: call goabiinternal void @"runtime.deferprocStack<builtin.
// LLVM: store volatile ptr {{.*}}, ptr [[STACK_SLOT0]]
// LLVM: store volatile i8 1, ptr [[STACK_BITS]]
// LLVM: store volatile i64 7, ptr [[STACK_RESULT]]
// LLVM: [[STACK_RECOVER]]:
// LLVM: call goabiinternal void @"runtime.deferreturn<builtin.{{[0-9]+}}>"()
// LLVM: load volatile i64, ptr [[STACK_RESULT]]
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmDeferStack(i64 %value)
// LLVM-OPT: [[STACK_OPT_SLOTS:%.*]] = alloca [1 x ptr], align 8, !goallc.open_defer_slots
// LLVM-OPT: [[STACK_OPT_BITS:%.*]] = alloca i8, {{.*}}!goallc.open_defer_bits
// LLVM-OPT: [[STACK_OPT_RESULT:%.*]] = alloca i64
// LLVM-OPT: store volatile i64 0, ptr [[STACK_OPT_RESULT]]
// LLVM-OPT: callbr void @llvm.go.defer.edge()
// LLVM-OPT-NEXT: to label %{{.*}} [label %[[STACK_OPT_RECOVER:.*]]]
// LLVM-OPT: load volatile i64, ptr [[STACK_OPT_RESULT]]
// LLVM-OPT-NOT: call goabiinternal void @"runtime.deferprocStack<builtin.
// LLVM-OPT: store volatile ptr {{.*}}, ptr [[STACK_OPT_SLOTS]]
// LLVM-OPT: store volatile i8 1, ptr [[STACK_OPT_BITS]]
// LLVM-OPT: [[STACK_OPT_RECOVER]]:
// LLVM-OPT: call goabiinternal void @"runtime.deferreturn<builtin.{{[0-9]+}}>"()

// LLVM-LABEL: define goabiinternal void @codegen.llvmDeferHeap(i64 %count)
// LLVM: [[HEAP_NORMAL_RETURN:[A-Za-z0-9_.]+]]:
// LLVM: call goabiinternal void @"runtime.deferreturn<builtin.{{[0-9]+}}>"()
// LLVM: ret void
// LLVM: [[HEAP_RECOVER:[A-Za-z0-9_.]+]]:
// LLVM-NEXT: call goabiinternal void @"runtime.deferreturn<builtin.{{[0-9]+}}>"()
// LLVM: call goabiinternal void @"runtime.deferproc<builtin.{{[0-9]+}}>"(
// LLVM: callbr void @llvm.go.defer.edge()
// LLVM-NEXT: to label %{{.*}} [label %[[HEAP_RECOVER]]]
// LLVM: define goabiinternal void @codegen.llvmDeferHeap.deferwrap1({{.*}}) {{.*}}!goobj.func.info ![[WRAPPER_INFO:[0-9]+]]
// LLVM: define goabiinternal {{.*}} @codegen.llvmRecover(){{.*}} #[[LLVM_NOINLINE]] gc "goallc"
// LLVM: call goabiinternal {{.*}} @"runtime.gorecover<builtin.{{[0-9]+}}>"(
// LLVM-OPT-LABEL: define goabiinternal void @codegen.llvmDeferHeap(i64 %count)
// LLVM-OPT: [[HEAP_OPT_RECOVER:common.ret]]:
// LLVM-OPT-NEXT: call goabiinternal void @"runtime.deferreturn<builtin.{{[0-9]+}}>"()
// LLVM-OPT: call goabiinternal void @"runtime.deferproc<builtin.{{[0-9]+}}>"(
// LLVM-OPT: callbr void @llvm.go.defer.edge()
// LLVM-OPT-NEXT: to label %{{.*}} [label %[[HEAP_OPT_RECOVER]]]
// LLVM-OPT: define goabiinternal void @codegen.llvmDeferHeap.deferwrap1({{.*}}) {{.*}}!goobj.func.info ![[WRAPPER_OPT_INFO:[0-9]+]]
// LLVM-OPT: define goabiinternal {{.*}} @codegen.llvmRecover(){{.*}} #[[LLVM_NOINLINE]] gc "goallc"
// LLVM-OPT: call goabiinternal {{.*}} @"runtime.gorecover<builtin.{{[0-9]+}}>"(

// An unnamed result still has a recovery-visible home. If evaluating a return
// expression panics, defer recovery must return the last committed value.
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmDeferUnnamedResult(i64 %value)
// LLVM: [[UNNAMED_RESULT:%.*]] = alloca i64, align 8{{$}}
// LLVM: store volatile i64 0, ptr [[UNNAMED_RESULT]]
// LLVM: call goabiinternal void @"runtime.deferreturn<builtin.{{[0-9]+}}>"()
// LLVM-NEXT: {{.*}} = load volatile i64, ptr [[UNNAMED_RESULT]]
// LLVM: attributes #[[LLVM_NOINLINE]] = { {{.*}}noinline
// LLVM: ![[WRAPPER_INFO]] = !{i8 23, i8 0}
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmDeferUnnamedResult(i64 %value)
// LLVM-OPT: [[UNNAMED_OPT_RESULT:%.*]] = alloca i64, align 8{{$}}
// LLVM-OPT: store volatile i64 0, ptr [[UNNAMED_OPT_RESULT]]
// LLVM-OPT: [[UNNAMED_OPT_RETURN:common.ret]]:
// LLVM-OPT-NEXT: {{.*}} = load volatile i64, ptr [[UNNAMED_OPT_RESULT]]
// LLVM-OPT: open.defer.recovery:
// LLVM-OPT-NEXT: call goabiinternal void @"runtime.deferreturn<builtin.{{[0-9]+}}>"()
// LLVM-OPT-NEXT: br label %[[UNNAMED_OPT_RETURN]]
// LLVM-OPT: attributes #[[LLVM_NOINLINE]] = { {{.*}}noinline
// LLVM-OPT: ![[WRAPPER_OPT_INFO]] = !{i8 23, i8 0}

// A defer in a loop uses runtime.deferproc rather than deferprocStack. Keep this
// function before llvmDeferStack because the compiler emits bodies in reverse
// declaration order.
func llvmDeferHeap(count int) {
	for i := 0; i < count; i++ {
		defer func(value int) {
			llvmDeferSink += value
		}(i)
	}
}

func llvmDeferStack(value int) (result int) {
	defer func() {
		result += value
	}()
	result = 7
	return
}

func llvmDeferPointerResult(pointer *int) (result *int) {
	defer func() {}()
	result = pointer
	panic(llvmDeferSink)
}

func llvmDeferUnnamedResult(value int) int {
	defer func() {}()
	return value
}

func llvmRecover() any {
	return recover()
}
