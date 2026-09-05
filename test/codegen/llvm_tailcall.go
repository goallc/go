// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-LABEL: define weak goabiinternal i64 @"codegen.(*llvmTailInter).dynamic"
// LLVM-SAME: !goobj.func.info
// LLVM: %[[INTER_CALL:[0-9]+]] = musttail call goabiinternal i64 %{{[0-9]+}}
// LLVM-NEXT: ret i64 %[[INTER_CALL]]
// LLVM-LABEL: define weak goabiinternal void @"codegen.(*llvmTailStack).stack"
// LLVM-SAME: ptr goret({{.*}}) {{.*}}%[[STACK_RESULT:[A-Za-z0-9_.]+]])
// LLVM-SAME: !goobj.func.info
// LLVM-NOT: @llvm.lifetime.start
// LLVM: musttail call goabiinternal void @"codegen.(*llvmTailStackBase).stack"
// LLVM-SAME: ptr {{.*}}goret({{.*}}) {{.*}}%[[STACK_RESULT]])
// LLVM-NEXT: ret void
// LLVM-LABEL: define weak goabiinternal i64 @"codegen.(*llvmTailStatic).static"
// LLVM-SAME: !goobj.func.info
// LLVM: %[[STATIC_CALL:[0-9]+]] = musttail call goabiinternal i64 @"codegen.(*llvmTailBase).static"
// LLVM-NEXT: ret i64 %[[STATIC_CALL]]
// LLVM-OPT-LABEL: define weak goabiinternal i64 @"codegen.(*llvmTailInter).dynamic"
// LLVM-OPT-SAME: !goobj.func.info
// LLVM-OPT: %[[INTER_OPT_CALL:[0-9]+]] = musttail call goabiinternal i64 %{{[0-9]+}}
// LLVM-OPT-NEXT: ret i64 %[[INTER_OPT_CALL]]
// LLVM-OPT-LABEL: define weak goabiinternal void @"codegen.(*llvmTailStack).stack"
// LLVM-OPT-SAME: ptr goret({{.*}}) {{.*}}%[[STACK_OPT_RESULT:[A-Za-z0-9_.]+]])
// LLVM-OPT-SAME: !goobj.func.info
// LLVM-OPT-NOT: @llvm.lifetime.start
// LLVM-OPT: musttail call goabiinternal void @"codegen.(*llvmTailStackBase).stack"
// LLVM-OPT-SAME: ptr {{.*}}goret({{.*}}) {{.*}}%[[STACK_OPT_RESULT]])
// LLVM-OPT-NEXT: ret void
// LLVM-OPT-LABEL: define weak goabiinternal i64 @"codegen.(*llvmTailStatic).static"
// LLVM-OPT-SAME: !goobj.func.info
// LLVM-OPT: %[[STATIC_OPT_CALL:[0-9]+]] = musttail call goabiinternal i64 @"codegen.(*llvmTailBase).static"
// LLVM-OPT-NEXT: ret i64 %[[STATIC_OPT_CALL]]

type llvmTailBase struct{}

//go:noinline
func (*llvmTailBase) static(value int) int {
	return value + 1
}

type llvmTailStatic struct {
	*llvmTailBase
}

// Force the compiler-generated static method wrapper.
var llvmTailStaticMethod = (*llvmTailStatic).static

type llvmTailStackValue [17]*int

type llvmTailStackBase struct{}

//go:noinline
func (*llvmTailStackBase) stack(value llvmTailStackValue) llvmTailStackValue {
	return value
}

type llvmTailStack struct {
	*llvmTailStackBase
}

// Force a method wrapper whose argument and result are both assigned to the
// Go ABI stack frame.
var llvmTailStackMethod = (*llvmTailStack).stack

type llvmTailInterface interface {
	dynamic(int) int
}

type llvmTailInter struct {
	llvmTailInterface
}

// Force the compiler-generated interface method wrapper.
var llvmTailInterMethod = (*llvmTailInter).dynamic
