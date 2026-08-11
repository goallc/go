// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

//go:noescape
func llvmCgoUnsafeSink(*uintptr)

// LLVM-LABEL: define goabi0 i64 @codegen.llvmCgoUnsafeFrame.goallc.abi0(
// LLVM-SAME: i64 %p) #[[NOINLINE:[0-9]+]] gc "goallc"
// LLVM-NOT: alloca
// LLVM: [[FRAME:%.*]] = {{.*}}call ptr @llvm.go.abi0.frame()
// LLVM-NOT: llvm.addressofreturnaddress
// LLVM-NOT: llvm.sponentry
// LLVM: [[RESULT:%.*]] = getelementptr i8, ptr [[FRAME]], i64 8
// LLVM: store i64 %p, ptr [[FRAME]]
// LLVM: {{.*}}call goabiinternal void @codegen.llvmCgoUnsafeSink(ptr{{.*}} [[FRAME]])
// LLVM: {{%.*}} = load i64, ptr [[RESULT]]
// LLVM: attributes #[[NOINLINE]] = { {{.*}}noinline
// LLVM-OPT-LABEL: define goabi0 i64 @codegen.llvmCgoUnsafeFrame.goallc.abi0(
// LLVM-OPT-SAME: i64 %p) {{.*}}#[[OPT_NOINLINE:[0-9]+]] gc "goallc"
// LLVM-OPT-NOT: alloca
// LLVM-OPT: [[OPT_FRAME:%.*]] = {{.*}}call ptr @llvm.go.abi0.frame()
// LLVM-OPT-NOT: llvm.addressofreturnaddress
// LLVM-OPT-NOT: llvm.sponentry
// LLVM-OPT: [[OPT_RESULT:%.*]] = getelementptr i8, ptr [[OPT_FRAME]], i64 8
// LLVM-OPT: store i64 %p, ptr [[OPT_FRAME]]
// LLVM-OPT: {{.*}}call goabiinternal void @codegen.llvmCgoUnsafeSink(ptr{{.*}} [[OPT_FRAME]])
// LLVM-OPT: {{%.*}} = load i64, ptr [[OPT_RESULT]]
// LLVM-OPT: attributes #[[OPT_NOINLINE]] = { {{.*}}noinline
//
//go:cgo_unsafe_args
func llvmCgoUnsafeFrame(p uintptr) (r uintptr) {
	llvmCgoUnsafeSink(&p)
	return
}
