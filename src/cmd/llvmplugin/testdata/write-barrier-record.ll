target triple = "x86_64-unknown-linux-goobj"

declare void @goallc.gc.write.record(ptr, ptr, i32)
declare void @safepoint()

; The normal store remains visible to forwarding, while the GC record survives.
; OPT-LABEL: define {{.*}}ptr @forward(
; OPT: call void @goallc.gc.write.record(ptr %value, ptr %dst, i32 0)
; OPT-NEXT: store ptr %value, ptr %dst
; OPT-NEXT: ret ptr %value
; LOWER-LABEL: define {{.*}}ptr @forward(
; LOWER: load i32, ptr @runtime.writeBarrier
; LOWER: br i1
; LOWER: %wb.old = load i64, ptr %dst
; LOWER: call ptr @llvm.go.gc.write.barrier(i32 2)
; LOWER: store i64
; LOWER: store i64 %wb.old
; LOWER: store ptr %value, ptr %dst
; LOWER: ret ptr %value
define ptr @forward(ptr %dst, ptr %value) gc "goallc" {
  call void @goallc.gc.write.record(ptr %value, ptr %dst, i32 0)
  store ptr %value, ptr %dst
  %loaded = load ptr, ptr %dst
  ret ptr %loaded
}

; Adjacent records share one reservation and deduplicate the new pointer.
; LOWER-LABEL: define {{.*}}void @pair(
; LOWER: call ptr @llvm.go.gc.write.barrier(i32 3)
; LOWER-NOT: call ptr @llvm.go.gc.write.barrier
; LOWER: ret void
define void @pair(ptr %dst, ptr %value) gc "goallc" {
  call void @goallc.gc.write.record(ptr %value, ptr %dst, i32 0)
  store ptr %value, ptr %dst
  %next = getelementptr ptr, ptr %dst, i64 1
  call void @goallc.gc.write.record(ptr %value, ptr %next, i32 0)
  store ptr %value, ptr %next
  ret void
}

; The original Go zero proof permits omitting only the old-value read.
; LOWER-LABEL: define {{.*}}void @fresh(
; LOWER-NOT: %wb.old
; LOWER: call ptr @llvm.go.gc.write.barrier(i32 1)
; LOWER-NOT: %wb.old
; LOWER: ret void
define void @fresh(ptr %dst, ptr %value) gc "goallc" {
  call void @goallc.gc.write.record(ptr %value, ptr %dst, i32 1)
  store ptr %value, ptr %dst
  ret void
}

; A safepoint separates reservations; never hoist the second record over it.
; LOWER-LABEL: define {{.*}}void @separate(
; LOWER: call ptr @llvm.go.gc.write.barrier(i32 2)
; LOWER: call void @safepoint()
; LOWER: call ptr @llvm.go.gc.write.barrier(i32 2)
; LOWER: ret void
define void @separate(ptr %dst, ptr %value, ptr %other) gc "goallc" {
  call void @goallc.gc.write.record(ptr %value, ptr %dst, i32 0)
  store ptr %value, ptr %dst
  call void @safepoint()
  call void @goallc.gc.write.record(ptr %other, ptr %dst, i32 0)
  store ptr %other, ptr %dst
  ret void
}


; Five unrelated writes require two reservations, each within the runtime limit.
; LIMIT-LABEL: define void @limit(
; LIMIT: call ptr @llvm.go.gc.write.barrier(i32 8)
; LIMIT: call ptr @llvm.go.gc.write.barrier(i32 2)
; LIMIT-NOT: call ptr @llvm.go.gc.write.barrier
; LIMIT: ret void
define void @limit(ptr %a, ptr %b, ptr %c, ptr %d, ptr %e,
                   ptr %v, ptr %w, ptr %x, ptr %y, ptr %z) gc "goallc" {
  call void @goallc.gc.write.record(ptr %v, ptr %a, i32 0)
  store ptr %v, ptr %a
  call void @goallc.gc.write.record(ptr %w, ptr %b, i32 0)
  store ptr %w, ptr %b
  call void @goallc.gc.write.record(ptr %x, ptr %c, i32 0)
  store ptr %x, ptr %c
  call void @goallc.gc.write.record(ptr %y, ptr %d, i32 0)
  store ptr %y, ptr %d
  call void @goallc.gc.write.record(ptr %z, ptr %e, i32 0)
  store ptr %z, ptr %e
  ret void
}

; Repeated destinations need the original old value and both new values.
; LIMIT-LABEL: define void @overwrite(
; LIMIT: %wb.old = load i64, ptr %dst
; LIMIT-NOT: load i64
; LIMIT: call ptr @llvm.go.gc.write.barrier(i32 3)
; LIMIT: ret void
define void @overwrite(ptr %dst, ptr %first, ptr %second) gc "goallc" {
  call void @goallc.gc.write.record(ptr %first, ptr %dst, i32 0)
  store ptr %first, ptr %dst
  call void @goallc.gc.write.record(ptr %second, ptr %dst, i32 0)
  store ptr %second, ptr %dst
  ret void
}

; Null stores record only the overwritten pointer, even without omission flags.
; LIMIT-LABEL: define void @clear(
; LIMIT: %wb.old = load i64, ptr %dst
; LIMIT: call ptr @llvm.go.gc.write.barrier(i32 1)
; LIMIT: store i64 %wb.old
; LIMIT: store ptr null, ptr %dst
; LIMIT: ret void
define void @clear(ptr %dst) gc "goallc" {
  call void @goallc.gc.write.record(ptr null, ptr %dst, i32 0)
  store ptr null, ptr %dst
  ret void
}

; SimplifyCFG merges calls whose omission flags differ. Dynamic flags must
; conservatively retain both entries rather than crashing in late lowering.
; OPT-LABEL: define {{.*}}void @merged_flags(
; OPT: select i1 %condition, i32 0, i32 2
; OPT: call void @goallc.gc.write.record
; LOWER-LABEL: define {{.*}}void @merged_flags(
; LOWER: load i64, ptr %dst
; LOWER: call ptr @llvm.go.gc.write.barrier(i32 2)
; LOWER: store ptr %value, ptr %dst
; LOWER: ret void
define void @merged_flags(ptr %dst, ptr %value, i1 %condition) gc "goallc" {
  br i1 %condition, label %a, label %b
a:
  call void @goallc.gc.write.record(ptr %value, ptr %dst, i32 0)
  br label %exit
b:
  call void @goallc.gc.write.record(ptr %value, ptr %dst, i32 2)
  br label %exit
exit:
  store ptr %value, ptr %dst
  ret void
}

; OPT: attributes {{.*}}"gc-leaf-function"
