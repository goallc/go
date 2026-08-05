// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

var llvmDeferSink int

// LLVM-LABEL: define goabiinternal ptr @codegen.llvmDeferPointerResult(ptr{{.*}} %pointer)
// LLVM: [[RESULT:%.*]] = alloca ptr, {{.*}}!goallc.defer_result
// LLVM: callbr void @llvm.go.defer.edge()
// LLVM-NEXT: to label %{{.*}} [label %[[RECOVER:[A-Za-z0-9_.]+]]]
// LLVM: [[RECOVER]]:
// LLVM-NEXT: call goabiinternal void @runtime.deferreturn()
// LLVM-NEXT: {{.*}} = load volatile ptr, ptr [[RESULT]]
// LLVM-OPT-LABEL: define goabiinternal ptr @codegen.llvmDeferPointerResult(ptr{{.*}} %pointer)
// LLVM-OPT: [[RESULT_OPT:%.*]] = alloca ptr, {{.*}}!goallc.defer_result
// LLVM-OPT: callbr void @llvm.go.defer.edge()
// LLVM-OPT-NEXT: to label %{{.*}} [label %[[RECOVER_OPT:[A-Za-z0-9_.]+]]]
// LLVM-OPT: [[RECOVER_OPT]]:
// LLVM-OPT-NEXT: call goabiinternal void @runtime.deferreturn()
// LLVM-OPT-NEXT: {{.*}} = load volatile ptr, ptr [[RESULT_OPT]]

// LLVM-LABEL: define goabiinternal i64 @codegen.llvmDeferStack(i64 %value)
// LLVM: call goabiinternal void @runtime.deferprocStack
// LLVM: callbr void @llvm.go.defer.edge()
// LLVM-NEXT: to label %[[STACK_NORMAL:.*]] [label %[[STACK_RECOVER:.*]]]
// LLVM: [[STACK_RECOVER]]:
// LLVM: call goabiinternal void @runtime.deferreturn()
// LLVM: load volatile i64
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmDeferStack(i64 %value)
// LLVM-OPT: call goabiinternal void @runtime.deferprocStack
// LLVM-OPT: callbr void @llvm.go.defer.edge()
// LLVM-OPT-NEXT: to label %{{.*}} [label %[[STACK_OPT_RECOVER:.*]]]
// LLVM-OPT: [[STACK_OPT_RECOVER]]:
// LLVM-OPT: call goabiinternal void @runtime.deferreturn()
// LLVM-OPT: load volatile i64

// LLVM-LABEL: define goabiinternal void @codegen.llvmDeferHeap(i64 %count)
// LLVM: [[HEAP_NORMAL_RETURN:[A-Za-z0-9_.]+]]:
// LLVM: call goabiinternal void @runtime.deferreturn()
// LLVM: ret void
// LLVM: [[HEAP_RECOVER:[A-Za-z0-9_.]+]]:
// LLVM-NEXT: call goabiinternal void @runtime.deferreturn()
// LLVM: call goabiinternal void @runtime.deferproc(
// LLVM: callbr void @llvm.go.defer.edge()
// LLVM-NEXT: to label %{{.*}} [label %[[HEAP_RECOVER]]]
// LLVM: define goabiinternal void @codegen.llvmDeferHeap.deferwrap1({{.*}}) {{.*}}!goobj.func.info ![[WRAPPER_INFO:[0-9]+]]
// LLVM: ![[WRAPPER_INFO]] = !{i8 23, i8 0}
// LLVM-OPT-LABEL: define goabiinternal void @codegen.llvmDeferHeap(i64 %count)
// LLVM-OPT: [[HEAP_OPT_RECOVER:common.ret]]:
// LLVM-OPT-NEXT: call goabiinternal void @runtime.deferreturn()
// LLVM-OPT: call goabiinternal void @runtime.deferproc(
// LLVM-OPT: callbr void @llvm.go.defer.edge()
// LLVM-OPT-NEXT: to label %{{.*}} [label %[[HEAP_OPT_RECOVER]]]
// LLVM-OPT: define goabiinternal void @codegen.llvmDeferHeap.deferwrap1({{.*}}) {{.*}}!goobj.func.info ![[WRAPPER_OPT_INFO:[0-9]+]]
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
