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
// LLVM-AMD64: [[P_FRAME:%.*]] = getelementptr i8, ptr {{%.*}}, i64 8
// LLVM-AMD64: [[R_FRAME:%.*]] = getelementptr i8, ptr {{%.*}}, i64 16
// LLVM-ARM64: [[P_FRAME:%.*]] = getelementptr i8, ptr {{%.*}}, i64 8
// LLVM-ARM64: [[R_FRAME:%.*]] = getelementptr i8, ptr {{%.*}}, i64 16
// LLVM: store i64 %p, ptr [[P_FRAME]]
// LLVM: call goabiinternal void @codegen.llvmCgoUnsafeSink(ptr [[P_FRAME]])
// LLVM: {{%.*}} = load i64, ptr [[R_FRAME]]
// LLVM: attributes #[[NOINLINE]] = { {{.*}}noinline
// LLVM-OPT-LABEL: define goabi0 i64 @codegen.llvmCgoUnsafeFrame.goallc.abi0(
// LLVM-OPT-SAME: i64 %p) {{.*}} #[[OPT_NOINLINE:[0-9]+]] gc "goallc"
// LLVM-OPT-NOT: alloca
// LLVM-OPT-AMD64: [[OPT_BASE:%.*]] = {{.*}}call ptr @llvm.addressofreturnaddress{{(.p0)?}}()
// LLVM-OPT-AMD64: [[OPT_P_FRAME:%.*]] = getelementptr i8, ptr [[OPT_BASE]], i64 8
// LLVM-OPT-AMD64: [[OPT_R_FRAME:%.*]] = getelementptr i8, ptr [[OPT_BASE]], i64 16
// LLVM-OPT-ARM64: [[OPT_BASE:%.*]] = {{.*}}call ptr @llvm.sponentry{{(.p0)?}}()
// LLVM-OPT-ARM64: [[OPT_P_FRAME:%.*]] = getelementptr i8, ptr [[OPT_BASE]], i64 8
// LLVM-OPT-ARM64: [[OPT_R_FRAME:%.*]] = getelementptr i8, ptr [[OPT_BASE]], i64 16
// LLVM-OPT: store i64 %p, ptr [[OPT_P_FRAME]]
// LLVM-OPT: call goabiinternal void @codegen.llvmCgoUnsafeSink(ptr {{.*}}[[OPT_P_FRAME]])
// LLVM-OPT: {{%.*}} = load i64, ptr [[OPT_R_FRAME]]
// LLVM-OPT: attributes #[[OPT_NOINLINE]] = { {{.*}}noinline
//
//go:cgo_unsafe_args
func llvmCgoUnsafeFrame(p uintptr) (r uintptr) {
	llvmCgoUnsafeSink(&p)
	return
}
