// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM: %codegen.llvmFrameRecord = type { i8, %codegen.llvmFrameTail }
// LLVM: %codegen.llvmFrameTail = type { ptr, [8 x i64] }
// LLVM-LABEL: define goabiinternal void @codegen.llvmFrameCopy(
// LLVM: [[TAIL:%.*]] = load %codegen.llvmFrameTail, ptr
// LLVM-NEXT: store %codegen.llvmFrameTail [[TAIL]], ptr
// LLVM: call goabiinternal void @codegen.llvmFrameCheckpoint()
// LLVM: [[RESULT:%.*]] = load %codegen.llvmFrameRecord, ptr
// LLVM-NEXT: store %codegen.llvmFrameRecord [[RESULT]], ptr %.result0
// LLVM: ret void
// LLVM-OPT-LABEL: define goabiinternal void @codegen.llvmFrameCopy(
// LLVM-OPT: [[ROOT:%.*]] = load ptr, ptr
// LLVM-OPT: call goabiinternal void @codegen.llvmFrameCheckpoint()
// LLVM-OPT: insertvalue %codegen.llvmFrameRecord {{.*}}, ptr [[ROOT]], 1, 0
// LLVM-OPT: store %codegen.llvmFrameRecord {{.*}}, ptr %.result0
// LLVM-OPT: ret void
//
// Defer's observable copies remain volatile. Only the subsequent value
// assignment from the completed snapshot uses an aggregate load/store.
// LLVM-LABEL: define goabiinternal void @codegen.llvmFrameDeferred(
// LLVM: call void @llvm.memmove.p0.p0.i64({{.*}}i64 80, i1 true)
// LLVM: call void @llvm.memmove.p0.p0.i64({{.*}}i64 80, i1 true)
// LLVM: [[DEFERRED:%.*]] = load %codegen.llvmFrameRecord, ptr {{.*}}.snapshot
// LLVM-NEXT: store %codegen.llvmFrameRecord [[DEFERRED]], ptr %.result0
//
// Both creating and consuming a value snapshot retain the original type.
// LLVM-LABEL: define goabiinternal ptr @codegen.llvmFrameSnapshot(
// LLVM: [[SNAPSHOT:%.*]] = alloca %codegen.llvmFrameRecord
// LLVM: [[INITIAL:%.*]] = load %codegen.llvmFrameRecord, ptr %source
// LLVM-NEXT: store %codegen.llvmFrameRecord [[INITIAL]], ptr %.result0
// LLVM: [[SAVED:%.*]] = load %codegen.llvmFrameRecord, ptr %.result0
// LLVM-NEXT: store %codegen.llvmFrameRecord [[SAVED]], ptr [[SNAPSHOT]]
// LLVM: [[RESTORED:%.*]] = load %codegen.llvmFrameRecord, ptr [[SNAPSHOT]]
// LLVM-NEXT: store %codegen.llvmFrameRecord [[RESTORED]], ptr %.result0
//
// Pointer-free copies remain byte copies, avoiding large first-class values.
// LLVM-LABEL: define goabiinternal void @codegen.llvmFrameBytes(
// LLVM-NOT: load [80 x i8]
// LLVM: call void @llvm.memmove.p0.p0.i64({{.*}}i64 80, i1 false)
// LLVM: ret void
//
// Heap assignment still records its bulk write barrier before the copy.
// LLVM-LABEL: define goabiinternal void @codegen.llvmFrameAssign(
// LLVM: call goabiinternal void @runtime.wbMove(
// LLVM: [[ASSIGN:%.*]] = load %codegen.llvmFrameRecord, ptr %src
// LLVM-NEXT: store %codegen.llvmFrameRecord [[ASSIGN]], ptr %dst
// LLVM: ret void
// LLVM-LABEL: define goabiinternal void @codegen.llvmFrameReturn(
// LLVM: [[DIRECT:%.*]] = load %codegen.llvmFrameRecord, ptr %source
// LLVM-NEXT: store %codegen.llvmFrameRecord [[DIRECT]], ptr %.result0
// LLVM: ret void

type llvmFrameTail struct {
	pointer *int
	words   [8]uintptr
}

type llvmFrameRecord struct {
	flag bool
	data llvmFrameTail
}

//go:noescape
func llvmFrameCheckpoint()

// A field copy followed by a safepoint and a whole-object result copy must
// retain the pointer in data. The ordinary struct type already describes its
// alignment padding; no separate byte-copy type is needed.
//
//go:noinline
func llvmFrameCopy(source *llvmFrameRecord) llvmFrameRecord {
	var result llvmFrameRecord
	result.flag = source.flag
	result.data = source.data
	llvmFrameCheckpoint()
	return result
}

//go:noinline
func llvmFrameReturn(source *llvmFrameRecord) llvmFrameRecord {
	return *source
}

//go:noinline
func llvmFrameAssign(dst, src *llvmFrameRecord) {
	*dst = *src
}

var llvmFrameEscaped **int

//go:noinline
func llvmFrameSnapshot(source *llvmFrameRecord) (result llvmFrameRecord, pointer *int) {
	result = *source
	llvmFrameEscaped = &pointer
	return
}

//go:noinline
func llvmFrameDeferred(source *llvmFrameRecord) (result llvmFrameRecord) {
	defer func() { result.flag = true }()
	result = *source
	return
}

//go:noinline
func llvmFrameBytes(src *[80]byte) [80]byte {
	return *src
}
