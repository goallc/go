// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-LABEL: define goabiinternal ptr @codegen.llvmWriteBarrierStore(
// LLVM-SAME: ptr{{[^%]*}}%dst, ptr{{[^%]*}}%[[VALUE:[a-zA-Z0-9._]+]])
// LLVM-SAME: #[[WBATTR:[0-9]+]] gc "goallc" {
// LLVM: load i32, ptr @runtime.writeBarrier
// LLVM: br i1
// LLVM: call ptr @llvm.go.gc.write.barrier(i32 2)
// LLVM: store ptr %[[VALUE]]
// LLVM: store ptr %[[VALUE]], ptr %dst
// LLVM-NOT: call ptr @llvm.go.gc.write.barrier
// LLVM: ret ptr %[[VALUE]]
// LLVM-OPT-LABEL: define goabiinternal ptr @codegen.llvmWriteBarrierStore(
// LLVM-OPT-SAME: ptr{{[^%]*}}%dst, ptr{{[^%]*}}%[[OPT_VALUE:[a-zA-Z0-9._]+]])
// LLVM-OPT: call ptr @llvm.go.gc.write.barrier(i32 2)
// LLVM-OPT-NOT: call ptr @llvm.go.gc.write.barrier
// LLVM-OPT: ret ptr %[[OPT_VALUE]]
func llvmWriteBarrierStore(dst **int, value *int) *int {
	local := value
	*dst = local
	return local
}

type llvmWriteBarrierPair struct {
	left  *int
	right *int
}

// The compiler drains its function queue in reverse source order, so keep the
// checks in emitted IR order rather than beside the two source declarations.
// LLVM-LABEL: define goabiinternal void @codegen.llvmWriteBarrierZero(
// LLVM-SAME: #[[WBATTR]] gc "goallc" {
// LLVM: call ptr @llvm.go.gc.write.barrier(i32 4)
// LLVM: store ptr null
// LLVM: store ptr null
// LLVM-LABEL: define goabiinternal void @codegen.llvmWriteBarrierMove(
// LLVM-SAME: #[[WBATTR]] gc "goallc" {
// LLVM: call ptr @llvm.go.gc.write.barrier(i32 4)
// LLVM: store ptr
// LLVM: store ptr
// LLVM: attributes #[[WBATTR]] = { {{.*}}"go-async-unsafe"{{.*}} }
func llvmWriteBarrierMove(dst, src *llvmWriteBarrierPair) {
	*dst = *src
}

func llvmWriteBarrierZero(dst *llvmWriteBarrierPair) {
	*dst = llvmWriteBarrierPair{}
}
