// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

//go:noescape
func llvmCgoUnsafeSink(*uintptr)

// LLVM-LABEL: define goabi0 void @"codegen.llvmCgoUnsafeFrame<ABI0>"(
// LLVM-SAME: ptr preallocated(i64) align 8 %p, ptr preallocated(i64) align 8 %q, ptr goret(i64) align 8 "goretindex"="0" [[RETURN:%.*]]) #[[NOINLINE:[0-9]+]] gc "goallc"
// LLVM-NOT: alloca
// LLVM: [[FRAME:%.*]] = {{.*}}call ptr @llvm.go.abi0.frame()
// LLVM-NOT: llvm.addressofreturnaddress
// LLVM-NOT: llvm.sponentry
// LLVM: [[RESULT:%.*]] = getelementptr i8, ptr [[FRAME]], i64 16
// LLVM: {{.*}}call goabiinternal void @codegen.llvmCgoUnsafeSink(ptr{{.*}} %p)
// LLVM: call void @llvm.memmove.p0.p0.i64(ptr align 8 [[RETURN]], ptr align 8 [[RESULT]], i64 8, i1 false)
// LLVM: ret void
// LLVM: attributes #[[NOINLINE]] = { {{.*}}noinline
// LLVM-OPT-LABEL: define goabi0 void @"codegen.llvmCgoUnsafeFrame<ABI0>"(
// LLVM-OPT-SAME: ptr preallocated(i64) align 8 %p, ptr preallocated(i64) align 8{{.*}} %q, ptr {{.*}}goret(i64) align 8{{.*}} "goretindex"="0" [[OPT_RETURN:%.*]]) {{.*}}#[[OPT_NOINLINE:[0-9]+]] gc "goallc"
// LLVM-OPT-NOT: alloca
// LLVM-OPT: [[OPT_FRAME:%.*]] = {{.*}}call ptr @llvm.go.abi0.frame()
// LLVM-OPT-NOT: llvm.addressofreturnaddress
// LLVM-OPT-NOT: llvm.sponentry
// LLVM-OPT: [[OPT_RESULT:%.*]] = getelementptr i8, ptr [[OPT_FRAME]], i64 16
// LLVM-OPT: {{.*}}call goabiinternal void @codegen.llvmCgoUnsafeSink(ptr{{.*}} %p)
// LLVM-OPT: [[OPT_VALUE:%.*]] = load i64, ptr [[OPT_RESULT]], align 8
// LLVM-OPT-NEXT: store i64 [[OPT_VALUE]], ptr [[OPT_RETURN]], align 8
// LLVM-OPT: ret void
// LLVM-OPT: attributes #[[OPT_NOINLINE]] = { {{.*}}noinline
//
//go:cgo_unsafe_args
func llvmCgoUnsafeFrame(p, q uintptr) (r uintptr) {
	llvmCgoUnsafeSink(&p)
	return
}
