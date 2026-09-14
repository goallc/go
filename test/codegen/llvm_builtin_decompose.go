// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// Expose the static interface data before write-barrier analysis. Only the
// overwritten data pointer needs to be recorded; the new pointer is global.
// LLVM-LABEL: define goabiinternal void @codegen.llvmBuiltinStaticInterface(
// LLVM-NOT: @llvm.go.gc.write.barrier(i32 2)
// LLVM: [[OLD:%.*]] = load i64, ptr [[DATA:%[^,]+]],
// LLVM: [[BUF:%.*]] = call ptr @llvm.go.gc.write.barrier(i32 1)
// LLVM: [[SLOT:%.*]] = getelementptr i8, ptr [[BUF]], i64 0
// LLVM: store i64 [[OLD]], ptr [[SLOT]],
// LLVM: [[DATA]] = getelementptr i8, ptr %dst, i64 8
// LLVM-NOT: @llvm.go.gc.write.barrier(i32 2)
// LLVM-OPT-LABEL: define goabiinternal void @codegen.llvmBuiltinStaticInterface(
// LLVM-OPT-NOT: @llvm.go.gc.write.barrier(i32 2)
// LLVM-OPT: call ptr @llvm.go.gc.write.barrier(i32 1)
// LLVM-OPT-NOT: @llvm.go.gc.write.barrier(i32 2)
func llvmBuiltinStaticInterface(dst *any) {
	*dst = "static"
}
