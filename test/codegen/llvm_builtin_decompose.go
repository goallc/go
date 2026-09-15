// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// Expose the static interface data before write-barrier analysis. Only the
// overwritten data pointer needs to be recorded; the new pointer is global.
// LLVM-LABEL: define goabiinternal void @codegen.llvmBuiltinStaticInterface(
// LLVM: [[DATA:%[^ ]+]] = getelementptr i8, ptr %dst, i64 8
// LLVM: call void @goallc.gc.write.record(ptr [[STATIC:@[^,]+]], ptr [[DATA]], i32 2)
// LLVM-NEXT: store ptr [[STATIC]], ptr [[DATA]],
// LLVM-OPT-LABEL: define goabiinternal void @codegen.llvmBuiltinStaticInterface(
// LLVM-OPT: [[OPT_DATA:%[^ ]+]] = getelementptr i8, ptr %dst, i64 8
// LLVM-OPT: call void @goallc.gc.write.record(ptr {{(nonnull )?}}[[OPT_STATIC:@[^,]+]], ptr [[OPT_DATA]], i32 2)
// LLVM-OPT-NEXT: store ptr [[OPT_STATIC]], ptr [[OPT_DATA]],
// The reservation is now expanded after the optimized IR dump. Check its
// capacity in the final object so the old-pointer-only invariant stays tested.
// LLVM-ASM-LABEL: TEXT codegen.llvmBuiltinStaticInterface(SB)
// LLVM-ASM-NOT: runtime.gcWriteBarrier2
// LLVM-ASM: runtime.gcWriteBarrier1
// LLVM-ASM-NOT: runtime.gcWriteBarrier2
func llvmBuiltinStaticInterface(dst *any) {
	*dst = "static"
}
