// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

//go:noescape
func llvmCgoUnsafeSink(*uintptr)

// LLVM-LABEL: define goabi0 void @"codegen.llvmCgoUnsafeFrame<ABI0>"(
// LLVM-SAME: ptr byval(i64) align 8 %p, ptr byval(i64) align 8 %q, ptr goret(i64) align 8 "goretindex"="0" [[RESULT_HOME:%[^)]+]]) #[[NOINLINE:[0-9]+]] gc "goallc"
// LLVM-NOT: alloca
// LLVM: [[FRAME:%.*]] = {{.*}}call ptr @llvm.go.abi0.frame()
// LLVM-NOT: llvm.addressofreturnaddress
// LLVM-NOT: llvm.sponentry
// LLVM: [[Q:%.*]] = getelementptr i8, ptr [[FRAME]], i64 8
// LLVM: [[RESULT:%.*]] = getelementptr i8, ptr [[FRAME]], i64 16
// LLVM: [[P_VALUE:%.*]] = load i64, ptr %p
// LLVM: store i64 [[P_VALUE]], ptr [[FRAME]]
// LLVM: [[Q_VALUE:%.*]] = load i64, ptr %q
// LLVM: store i64 [[Q_VALUE]], ptr [[Q]]
// LLVM: {{.*}}call goabiinternal void @codegen.llvmCgoUnsafeSink(ptr{{.*}} [[FRAME]])
// LLVM: {{%.*}} = load i64, ptr [[RESULT]]
// LLVM: call void @llvm.memmove{{.*}}(ptr align 8 [[RESULT_HOME]], ptr align 8 [[RESULT]], i64 8, i1 false)
// LLVM-OPT-LABEL: define goabi0 void @"codegen.llvmCgoUnsafeFrame<ABI0>"(
// LLVM-OPT-SAME: ptr{{.*}}byval(i64) align 8{{.*}} %p, ptr{{.*}}byval(i64) align 8{{.*}} %q, ptr{{.*}}goret(i64) align 8{{.*}} "goretindex"="0" [[OPT_RESULT_HOME:%[^)]+]]) {{.*}}#[[OPT_NOINLINE:[0-9]+]] gc "goallc"
// LLVM-OPT-NOT: alloca
// LLVM-OPT: [[OPT_FRAME:%.*]] = {{.*}}call ptr @llvm.go.abi0.frame()
// LLVM-OPT-NOT: llvm.addressofreturnaddress
// LLVM-OPT-NOT: llvm.sponentry
// LLVM-OPT: [[OPT_Q:%.*]] = getelementptr i8, ptr [[OPT_FRAME]], i64 8
// LLVM-OPT: [[OPT_RESULT:%.*]] = getelementptr i8, ptr [[OPT_FRAME]], i64 16
// LLVM-OPT: [[OPT_P_VALUE:%.*]] = load i64, ptr %p
// LLVM-OPT: store i64 [[OPT_P_VALUE]], ptr [[OPT_FRAME]]
// LLVM-OPT: [[OPT_Q_VALUE:%.*]] = load i64, ptr %q
// LLVM-OPT: store i64 [[OPT_Q_VALUE]], ptr [[OPT_Q]]
// LLVM-OPT: {{.*}}call goabiinternal void @codegen.llvmCgoUnsafeSink(ptr{{.*}} [[OPT_FRAME]])
// LLVM-OPT: [[OPT_RESULT_VALUE:%.*]] = load i64, ptr [[OPT_RESULT]]
// LLVM-OPT-NEXT: store i64 [[OPT_RESULT_VALUE]], ptr [[OPT_RESULT_HOME]], align 8
//
//go:cgo_unsafe_args
func llvmCgoUnsafeFrame(p, q uintptr) (r uintptr) {
	llvmCgoUnsafeSink(&p)
	return
}

// LLVM-LABEL: define goabiinternal i64 @codegen.llvmCgoUnsafeCall()
// LLVM: store i64 1, ptr [[CALL_P:%[^, ]+]]
// LLVM: store i64 2, ptr [[CALL_Q:%[^, ]+]]
// LLVM: call goabi0 void @"codegen.llvmCgoUnsafeFrame<ABI0>"(
// LLVM-SAME: ptr byval(i64) align 8 [[CALL_P]], ptr byval(i64) align 8 [[CALL_Q]], ptr goret(i64) align 8 "goretindex"="0" [[CALL_RESULT:%[^)]+]])
// LLVM: {{%.*}} = load i64, ptr [[CALL_RESULT]], align 8
// LLVM: attributes #[[NOINLINE]] = { {{.*}}noinline
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmCgoUnsafeCall()
// LLVM-OPT: store i64 1, ptr [[OPT_CALL_P:%[^, ]+]]
// LLVM-OPT: store i64 2, ptr [[OPT_CALL_Q:%[^, ]+]]
// LLVM-OPT: call goabi0 void @"codegen.llvmCgoUnsafeFrame<ABI0>"(
// LLVM-OPT-SAME: ptr {{.*}}byval(i64) align 8{{.*}} [[OPT_CALL_P]], ptr {{.*}}byval(i64) align 8{{.*}} [[OPT_CALL_Q]], ptr {{.*}}goret(i64) align 8{{.*}} "goretindex"="0" [[OPT_CALL_RESULT:%[^)]+]])
// LLVM-OPT: {{%.*}} = load i64, ptr [[OPT_CALL_RESULT]], align 8
// LLVM-OPT: attributes #[[OPT_NOINLINE]] = { {{.*}}noinline
func llvmCgoUnsafeCall() uintptr {
	return llvmCgoUnsafeFrame(1, 2)
}
