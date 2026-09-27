target datalayout = "e-p:64:64"
target triple = "x86_64-unknown-linux-goobj"

declare void @goallc.gc.write.record(ptr, ptr, i32)
declare noalias ptr @allocate(i64) allocsize(0)
declare void @modify(ptr)

; No frontend omission flag, and beyond Go SSA's 64-word zero map.
; CHECK-LABEL: define {{.*}}ptr @fresh_field(
; CHECK-NOT: wb.old
; CHECK: call ptr @llvm.go.gc.write.barrier(i32 1)
; CHECK-NOT: wb.old
; CHECK: ret ptr
define ptr @fresh_field(ptr %value) gc "goallc" {
  %obj = call ptr @allocate(i64 1024) allockind("alloc,zeroed")
  %dst = getelementptr i8, ptr %obj, i64 768
  call void @goallc.gc.write.record(ptr %value, ptr %dst, i32 0)
  store ptr %value, ptr %dst
  ret ptr %obj
}

; A disjoint field store does not destroy the remaining zero proof.
; CHECK-LABEL: define {{.*}}ptr @other_field(
; CHECK-NOT: wb.old
; CHECK: call ptr @llvm.go.gc.write.barrier(i32 1)
; CHECK-NOT: wb.old
; CHECK: ret ptr
define ptr @other_field(ptr %value) gc "goallc" {
  %obj = call ptr @allocate(i64 16) allockind("alloc,zeroed")
  store i64 42, ptr %obj
  %dst = getelementptr i8, ptr %obj, i64 8
  call void @goallc.gc.write.record(ptr %value, ptr %dst, i32 0)
  store ptr %value, ptr %dst
  ret ptr %obj
}

; Both values are null: the record can disappear entirely.
; CHECK-LABEL: define {{.*}}ptr @clear_fresh(
; CHECK-NOT: @runtime.writeBarrier
; CHECK-NOT: @llvm.go.gc.write.barrier
; CHECK: ret ptr
define ptr @clear_fresh() gc "goallc" {
  %obj = call ptr @allocate(i64 8) allockind("alloc,zeroed")
  call void @goallc.gc.write.record(ptr null, ptr %obj, i32 0)
  store ptr null, ptr %obj
  ret ptr %obj
}

; Explicit null stores can also establish zero, independently of allocation.
; CHECK-LABEL: define {{.*}}void @stored_null(
; CHECK-NOT: wb.old
; CHECK: call ptr @llvm.go.gc.write.barrier(i32 1)
; CHECK-NOT: wb.old
; CHECK: ret void
define void @stored_null(ptr %dst, ptr %value) gc "goallc" {
  store ptr null, ptr %dst
  call void @goallc.gc.write.record(ptr %value, ptr %dst, i32 0)
  store ptr %value, ptr %dst
  ret void
}

; An allocation without the zeroed attribute provides no initial-value proof.
; CHECK-LABEL: define {{.*}}ptr @not_zeroed(
; CHECK: call ptr @llvm.go.gc.write.barrier(i32 2)
; CHECK: wb.old = load i64
; CHECK: ret ptr
define ptr @not_zeroed(ptr %value) gc "goallc" {
  %obj = call ptr @allocate(i64 8) allockind("alloc")
  call void @goallc.gc.write.record(ptr %value, ptr %obj, i32 0)
  store ptr %value, ptr %obj
  ret ptr %obj
}

; The initial contents cannot be reused after an overlapping write.
; CHECK-LABEL: define {{.*}}ptr @overwritten(
; CHECK: call ptr @llvm.go.gc.write.barrier(i32 2)
; CHECK: wb.old = load i64
; CHECK: ret ptr
define ptr @overwritten(ptr %old, ptr %value) gc "goallc" {
  %obj = call ptr @allocate(i64 8) allockind("alloc,zeroed")
  store ptr %old, ptr %obj
  call void @goallc.gc.write.record(ptr %value, ptr %obj, i32 0)
  store ptr %value, ptr %obj
  ret ptr %obj
}

; Nor after passing the allocation to an unknown function.
; CHECK-LABEL: define {{.*}}ptr @escaped(
; CHECK: call void @modify
; CHECK: call ptr @llvm.go.gc.write.barrier(i32 2)
; CHECK: wb.old = load i64
; CHECK: ret ptr
define ptr @escaped(ptr %value) gc "goallc" {
  %obj = call ptr @allocate(i64 8) allockind("alloc,zeroed")
  call void @modify(ptr %obj)
  call void @goallc.gc.write.record(ptr %value, ptr %obj, i32 0)
  store ptr %value, ptr %obj
  ret ptr %obj
}

; An unknown offset may overlap the destination.
; CHECK-LABEL: define {{.*}}ptr @may_alias(
; CHECK: call ptr @llvm.go.gc.write.barrier(i32 2)
; CHECK: wb.old = load i64
; CHECK: ret ptr
define ptr @may_alias(ptr %old, ptr %value, i64 %index) gc "goallc" {
  %obj = call ptr @allocate(i64 16) allockind("alloc,zeroed")
  %other = getelementptr ptr, ptr %obj, i64 %index
  store ptr %old, ptr %other
  call void @goallc.gc.write.record(ptr %value, ptr %obj, i32 0)
  store ptr %value, ptr %obj
  ret ptr %obj
}

; A null store to only part of the pointer is insufficient.
; CHECK-LABEL: define {{.*}}void @partial_zero(
; CHECK: call ptr @llvm.go.gc.write.barrier(i32 2)
; CHECK: wb.old = load i64
; CHECK: ret void
define void @partial_zero(ptr %dst, ptr %value) gc "goallc" {
  store i32 0, ptr %dst
  call void @goallc.gc.write.record(ptr %value, ptr %dst, i32 0)
  store ptr %value, ptr %dst
  ret void
}

; A mutation on just one predecessor must invalidate the allocation proof.
; CHECK-LABEL: define {{.*}}ptr @conditional_write(
; CHECK: call ptr @llvm.go.gc.write.barrier(i32 2)
; CHECK: wb.old = load i64
; CHECK: ret ptr
define ptr @conditional_write(ptr %value, i1 %condition) gc "goallc" {
entry:
  %obj = call ptr @allocate(i64 8) allockind("alloc,zeroed")
  br i1 %condition, label %change, label %join
change:
  call void @modify(ptr %obj)
  br label %join
join:
  call void @goallc.gc.write.record(ptr %value, ptr %obj, i32 0)
  store ptr %value, ptr %obj
  ret ptr %obj
}

; Optimizer inlining can expose a zeroed allocation not visible to Go SSA.
; CHECK-LABEL: define {{.*}}ptr @after_inline(
; CHECK-NOT: wb.old
; CHECK: call ptr @llvm.go.gc.write.barrier(i32 1)
; CHECK-NOT: wb.old
; CHECK: ret ptr
define internal ptr @make_object() alwaysinline gc "goallc" {
  %obj = call ptr @allocate(i64 8) allockind("alloc,zeroed")
  ret ptr %obj
}
define ptr @after_inline(ptr %value) gc "goallc" {
  %obj = call ptr @make_object()
  call void @goallc.gc.write.record(ptr %value, ptr %obj, i32 0)
  store ptr %value, ptr %obj
  ret ptr %obj
}

; The query follows memory dependencies across blocks without losing the
; allocation proof to an unrelated write. Both successors must keep it.
; CHECK-LABEL: define {{.*}}ptr @across_blocks(
; CHECK-NOT: wb.old
; CHECK: call ptr @llvm.go.gc.write.barrier(i32 1)
; CHECK-NOT: wb.old
; CHECK: ret ptr
define ptr @across_blocks(ptr %unrelated, ptr %value, i1 %condition) gc "goallc" {
entry:
  %obj = call ptr @allocate(i64 8) allockind("alloc,zeroed")
  br i1 %condition, label %change, label %join
change:
  store volatile i64 42, ptr %unrelated
  br label %join
join:
  call void @goallc.gc.write.record(ptr %value, ptr %obj, i32 0)
  store ptr %value, ptr %obj
  ret ptr %obj
}

; A loop-carried write must not reuse the original allocation contents.
; CHECK-LABEL: define {{.*}}ptr @loop_write(
; CHECK: call ptr @llvm.go.gc.write.barrier(i32 2)
; CHECK: wb.old = load i64
; CHECK: ret ptr
define ptr @loop_write(ptr %value, i64 %count) gc "goallc" {
entry:
  %obj = call ptr @allocate(i64 8) allockind("alloc,zeroed")
  br label %loop
loop:
  %i = phi i64 [ 0, %entry ], [ %next, %loop ]
  call void @goallc.gc.write.record(ptr %value, ptr %obj, i32 0)
  store ptr %value, ptr %obj
  %next = add i64 %i, 1
  %again = icmp ult i64 %next, %count
  br i1 %again, label %loop, label %exit
exit:
  ret ptr %obj
}
