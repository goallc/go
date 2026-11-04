target triple = "aarch64-apple-darwin-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i8:8:32-i16:16:32-i64:64-i128:128-v128:64:64-n32:64-S128-Fn32"
%tail = type { ptr, [8 x i64] }
%record = type { i8, %tail }
declare void @llvm.memmove.p0.p0.i64(ptr, ptr, i64, i1 immarg)
declare void @llvm.memset.p0.i64(ptr, i8, i64, i1 immarg)
declare goabiinternal void @checkpoint()
declare goabiinternal void @observe(ptr)
; PREP-LABEL: define goabiinternal void @copy(
; PREP: load <{ i8, [7 x i8], <{ ptr, [8 x i64] }> }>, ptr %input, align 8
; PREP: store <{ i8, [7 x i8], <{ ptr, [8 x i64] }> }>
; PREP-NOT: call void @llvm.memmove
;
; IR-LABEL: define goabiinternal void @copy(
; IR: %[[ROOT:.*]] = load ptr, ptr
; IR: @checkpoint{{.*}}"gc-live"(ptr %[[ROOT]], ptr %out)
; IR: %[[RELOC:.*]] = call coldcc ptr @llvm.experimental.gc.relocate
; IR: store ptr %[[RELOC]], ptr
;
; Without preparation, SROA slices [1,80) into a byte array, then splits
; its first eight bytes through the pointer at offset 8 on AArch64. Even a
; target that keeps the copy whole has lost that pointer's GC identity.
; BASELINE: error: GoALLC cannot determine pointer alloca live-out activity

define goabiinternal void @copy(ptr %input) gc "goallc" {
entry:
  %src = alloca %record, align 8
  %tmp = alloca %record, align 8
  %out = alloca %record, align 8
  call void @llvm.memmove.p0.p0.i64(ptr align 8 %src, ptr align 8 %input, i64 80, i1 false)
  call void @llvm.memset.p0.i64(ptr align 8 %tmp, i8 0, i64 80, i1 false)
  %flag = load i8, ptr %src
  %flag.bit = icmp ne i8 %flag, 0
  %flag.byte = zext i1 %flag.bit to i8
  store i8 %flag.byte, ptr %tmp
  %src.tail = getelementptr %record, ptr %src, i32 0, i32 1
  %tmp.tail = getelementptr %record, ptr %tmp, i32 0, i32 1
  call void @llvm.memmove.p0.p0.i64(ptr align 8 %tmp.tail, ptr align 8 %src.tail, i64 72, i1 false)
  call goabiinternal void @checkpoint()
  call void @llvm.memmove.p0.p0.i64(ptr align 8 %out, ptr align 8 %tmp, i64 80, i1 false)
  call goabiinternal void @observe(ptr %out)
  ret void
}

; The same split must not reach a goret home. Before the assignment the result
; contents are not initialized; afterwards its complete pointer map is active.
; IR-LABEL: define goabiinternal void @copy_result(
; IR: @checkpoint{{.*}}[ "gc-live"(ptr %{{[^,]+}}, ptr %result) ]
; IR: @checkpoint{{.*}}ptr %result, i64 81, i64 2,
; PREP-LABEL: define goabiinternal void @copy_result(
; PREP: load <{ i8, [7 x i8], <{ ptr, [8 x i64] }> }>
; PREP: store <{ i8, [7 x i8], <{ ptr, [8 x i64] }> }>
; PREP-NOT: call void @llvm.memmove

define goabiinternal void @copy_result(ptr %input, ptr goret(%record) align 8 "goretindex"="0" %result) gc "goallc" {
entry:
  %src = alloca %record, align 8
  %tmp = alloca %record, align 8
  call void @llvm.memmove.p0.p0.i64(ptr align 8 %src, ptr align 8 %input, i64 80, i1 false)
  call void @llvm.memset.p0.i64(ptr align 8 %tmp, i8 0, i64 80, i1 false)
  %flag = load i8, ptr %src
  %flag.bit = icmp ne i8 %flag, 0
  %flag.byte = zext i1 %flag.bit to i8
  store i8 %flag.byte, ptr %tmp
  %src.tail = getelementptr %record, ptr %src, i32 0, i32 1
  %tmp.tail = getelementptr %record, ptr %tmp, i32 0, i32 1
  call void @llvm.memmove.p0.p0.i64(ptr align 8 %tmp.tail, ptr align 8 %src.tail, i64 72, i1 false)
  call goabiinternal void @checkpoint()
  call void @llvm.memmove.p0.p0.i64(ptr align 8 %result, ptr align 8 %tmp, i64 80, i1 false)
  call goabiinternal void @checkpoint()
  ret void
}

; Use different alignments and overlap two array elements. The complete
; snapshot must be loaded before writing the destination, including all
; seven padding bytes in each element. A memcpy is accepted as well.
%pair = type { i8, ptr }
; PREP-LABEL: define goabiinternal void @overlap(
; PREP: %[[SNAP:.*]] = load [2 x <{ i8, [7 x i8], ptr }>], ptr %storage, align 1
; PREP-NEXT: store [2 x <{ i8, [7 x i8], ptr }>] %[[SNAP]], ptr %next, align 2
; PREP: %[[COPY:.*]] = load [2 x <{ i8, [7 x i8], ptr }>], ptr %next, align 2
; PREP-NEXT: store [2 x <{ i8, [7 x i8], ptr }>] %[[COPY]], ptr %output, align 1
; PREP-NOT: call void @llvm.mem
; IR-LABEL: define goabiinternal void @overlap(
; IR: @llvm.experimental.gc.statepoint

define goabiinternal void @overlap(ptr byval([3 x %pair]) align 8 %storage, ptr %output) gc "goallc" {
entry:
  %next = getelementptr i8, ptr %storage, i64 16
  call void @llvm.memmove.p0.p0.i64(ptr align 2 %next, ptr align 1 %storage, i64 32, i1 false)
  call void @llvm.memcpy.p0.p0.i64(ptr align 1 %output, ptr align 2 %next, i64 32, i1 false)
  call goabiinternal void @observe(ptr %output)
  ret void
}
declare void @llvm.memcpy.p0.p0.i64(ptr, ptr, i64, i1 immarg)

; Do not invent a pointer interpretation for partial, volatile or variable
; byte copies, or change ordinary non-Go functions.
; PREP-LABEL: define goabiinternal void @untyped_copies(
; PREP: call void @llvm.memmove{{.*}}i64 3, i1 false
; PREP: call void @llvm.memmove{{.*}}i64 16, i1 true
; PREP: call void @llvm.memmove{{.*}}i64 %size, i1 false
; PREP: call void @llvm.memcpy.inline
; IR-LABEL: define goabiinternal void @untyped_copies(
define goabiinternal void @untyped_copies(ptr %input, i64 %size) gc "goallc" {
entry:
  %slot = alloca %pair, align 8
  call void @llvm.memmove.p0.p0.i64(ptr %slot, ptr %input, i64 3, i1 false)
  call void @llvm.memmove.p0.p0.i64(ptr %slot, ptr %input, i64 16, i1 true)
  call void @llvm.memmove.p0.p0.i64(ptr %slot, ptr %input, i64 %size, i1 false)
  call void @llvm.memcpy.inline.p0.p0.i64(ptr %slot, ptr %input, i64 16, i1 false)
  call goabiinternal void @observe(ptr %slot)
  ret void
}
declare void @llvm.memcpy.inline.p0.p0.i64(ptr, ptr, i64 immarg, i1 immarg)
; PREP-LABEL: define void @non_go(
; PREP: call void @llvm.memmove
; IR-LABEL: define void @non_go(
define void @non_go(ptr %input) {
entry:
  %slot = alloca %pair, align 8
  call void @llvm.memmove.p0.p0.i64(ptr %slot, ptr %input, i64 16, i1 false)
  call goabiinternal void @observe(ptr %slot)
  ret void
}
