// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "unsafe"

type llvmCgoZeroABI0 [0]byte

//go:noescape
func llvmCgoZeroSink(*uintptr)

// LLVM-LABEL: define goabi0 { [0 x i8], %go.abi.pad } @"codegen.llvmCgoZeroResult<ABI0>"
// LLVM-SAME: () #[[ZERO_RESULT_NOINLINE:[0-9]+]] gc "goallc"
// LLVM-NOT: alloca
// LLVM: [[ZERO_RESULT_FRAME:%.*]] = {{.*}}call ptr @llvm.go.abi0.frame()
// LLVM-NOT: store
// LLVM: {{.*}}call goabiinternal void @codegen.llvmCgoZeroSink(ptr{{.*}} [[ZERO_RESULT_FRAME]])
// LLVM: ret { [0 x i8], %go.abi.pad } undef
// LLVM-OPT-LABEL: define goabi0 { [0 x i8], %go.abi.pad } @"codegen.llvmCgoZeroResult<ABI0>"
// LLVM-OPT-SAME: () {{.*}}#[[ZERO_RESULT_OPT_NOINLINE:[0-9]+]] gc "goallc"
// LLVM-OPT-NOT: alloca
// LLVM-OPT: [[ZERO_RESULT_OPT_FRAME:%.*]] = {{.*}}call ptr @llvm.go.abi0.frame()
// LLVM-OPT-NOT: store
// LLVM-OPT: {{.*}}call goabiinternal void @codegen.llvmCgoZeroSink(ptr{{.*}} [[ZERO_RESULT_OPT_FRAME]])
// LLVM-OPT: ret { [0 x i8], %go.abi.pad } undef
//
//go:cgo_unsafe_args
func llvmCgoZeroResult() (r llvmCgoZeroABI0) {
	llvmCgoZeroSink((*uintptr)(unsafe.Pointer(&r)))
	return
}

// LLVM-LABEL: define goabi0 void @"codegen.llvmCgoZeroArgument<ABI0>"
// LLVM-SAME: ({ [0 x i8], %go.abi.pad } %v) #[[ZERO_ARGUMENT_NOINLINE:[0-9]+]] gc "goallc"
// LLVM-NOT: alloca
// LLVM: [[ZERO_ARGUMENT_FRAME:%.*]] = {{.*}}call ptr @llvm.go.abi0.frame()
// LLVM-NOT: store
// LLVM: {{.*}}call goabiinternal void @codegen.llvmCgoZeroSink(ptr{{.*}} [[ZERO_ARGUMENT_FRAME]])
// LLVM: ret void
// LLVM-OPT-LABEL: define goabi0 void @"codegen.llvmCgoZeroArgument<ABI0>"
// LLVM-OPT-SAME: ({ [0 x i8], %go.abi.pad } %v) {{.*}}#[[ZERO_ARGUMENT_OPT_NOINLINE:[0-9]+]] gc "goallc"
// LLVM-OPT-NOT: alloca
// LLVM-OPT: [[ZERO_ARGUMENT_OPT_FRAME:%.*]] = {{.*}}call ptr @llvm.go.abi0.frame()
// LLVM-OPT-NOT: store
// LLVM-OPT: {{.*}}call goabiinternal void @codegen.llvmCgoZeroSink(ptr{{.*}} [[ZERO_ARGUMENT_OPT_FRAME]])
// LLVM-OPT: ret void
//
//go:cgo_unsafe_args
func llvmCgoZeroArgument(v llvmCgoZeroABI0) {
	llvmCgoZeroSink((*uintptr)(unsafe.Pointer(&v)))
}
