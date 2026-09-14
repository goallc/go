target triple = "x86_64-unknown-linux-goobj"

@"runtime.writeBarrier<builtin.1>" = external global i8
@counter = internal global i64 0

declare ptr @llvm.go.gc.write.barrier(i32 immarg)
declare goabiinternal void @runtime.wbMove(ptr, ptr, ptr) #1

define goabiinternal i64 @write_barrier_range(ptr %dst, ptr %value) #0 gc "goallc" {
entry:
  %flag = load i32, ptr @"runtime.writeBarrier<builtin.1>", align 4
  %enabled = icmp ne i32 %flag, 0
  br i1 %enabled, label %slow, label %join

slow:
  %old = load ptr, ptr %dst, align 8
  %buf = call ptr @llvm.go.gc.write.barrier(i32 2)
  store ptr %value, ptr %buf, align 8
  %next = getelementptr i8, ptr %buf, i64 8
  store ptr %old, ptr %next, align 8
  br label %join

join:
  store ptr %value, ptr %dst, align 8
  br label %tail

tail:
  %result = load atomic i64, ptr @counter monotonic, align 8
  ret i64 %result
}

define goabiinternal i64 @bulk_write_barrier_range(ptr %type, ptr %dst, ptr %src) #0 gc "goallc" {
entry:
  %flag = load i32, ptr @"runtime.writeBarrier<builtin.1>", align 4
  %enabled = icmp ne i32 %flag, 0
  br i1 %enabled, label %slow, label %join

slow:
  call goabiinternal void @runtime.wbMove(ptr %type, ptr %dst, ptr %src) #1
  br label %join

join:
  %value = load ptr, ptr %src, align 8
  store ptr %value, ptr %dst, align 8
  br label %tail

tail:
  %result = load atomic i64, ptr @counter monotonic, align 8
  ret i64 %result
}

; An intrinsic without the compiler-emitted flag diamond is not a protocol the
; callback can delimit. It must retain the frontend's whole-function fallback.
define goabiinternal void @unrecognized_write_barrier(ptr %dst, ptr %value) #0 gc "goallc" {
entry:
  %buf = call ptr @llvm.go.gc.write.barrier(i32 2)
  store ptr %value, ptr %buf, align 8
  store ptr %value, ptr %dst, align 8
  ret void
}

attributes #0 = { "go-async-unsafe" }
attributes #1 = { "gc-leaf-function" }

; Each function has an unsafe stack-check prefix, a safe body prefix, an
; unsafe write-barrier protocol, and a safe tail. The separately laid-out
; morestack slow path may add another unsafe range later in the function.
; OBJVIEW-LABEL: "name": "write_barrier_range"
; OBJVIEW: "size": [[#WB_SIZE:]]
; OBJVIEW: "kind": "unsafe_point"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": [[#WB_CHECK_END:]]
; OBJVIEW-NEXT: "value": -2
; OBJVIEW: "start": [[#WB_CHECK_END]]
; OBJVIEW-NEXT: "end": [[#WB_BEGIN:]]
; OBJVIEW-NEXT: "value": -1
; OBJVIEW: "start": [[#WB_BEGIN]]
; OBJVIEW-NEXT: "end": [[#WB_END:]]
; OBJVIEW-NEXT: "value": -2
; OBJVIEW: "start": [[#WB_END]]
; OBJVIEW-NEXT: "end":
; OBJVIEW-NEXT: "value": -1
; OBJVIEW-LABEL: "name": "bulk_write_barrier_range"
; OBJVIEW: "size": [[#BULK_SIZE:]]
; OBJVIEW: "kind": "unsafe_point"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": [[#BULK_CHECK_END:]]
; OBJVIEW-NEXT: "value": -2
; OBJVIEW: "start": [[#BULK_CHECK_END]]
; OBJVIEW-NEXT: "end": [[#BULK_BEGIN:]]
; OBJVIEW-NEXT: "value": -1
; OBJVIEW: "start": [[#BULK_BEGIN]]
; OBJVIEW-NEXT: "end": [[#BULK_END:]]
; OBJVIEW-NEXT: "value": -2
; OBJVIEW: "start": [[#BULK_END]]
; OBJVIEW-NEXT: "end":
; OBJVIEW-NEXT: "value": -1
; OBJVIEW-LABEL: "name": "unrecognized_write_barrier"
; OBJVIEW: "size": [[#FALLBACK_SIZE:]]
; OBJVIEW: "kind": "unsafe_point"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": [[#FALLBACK_SIZE]]
; OBJVIEW-NEXT: "value": -2

; The condition depends on the flag through a cyclic PHI. Reachability must
; converge and preserve the safe tail rather than falling back to marking the
; entire function unsafe.
define goabiinternal i64 @cyclic_write_barrier(ptr %dst, ptr %value, i1 %again) #0 gc "goallc" {
entry:
  br label %loop
loop:
  %previous = phi i32 [ 0, %entry ], [ %flag, %latch ]
  %flag = load i32, ptr @"runtime.writeBarrier<builtin.1>", align 4
  %combined = or i32 %previous, %flag
  %enabled = icmp ne i32 %combined, 0
  br i1 %enabled, label %slow, label %join
slow:
  %old = load ptr, ptr %dst, align 8
  %buf = call ptr @llvm.go.gc.write.barrier(i32 2)
  store ptr %value, ptr %buf, align 8
  %next = getelementptr i8, ptr %buf, i64 8
  store ptr %old, ptr %next, align 8
  br label %join
join:
  store ptr %value, ptr %dst, align 8
  br i1 %again, label %latch, label %tail
latch:
  br label %loop
tail:
  %result = load atomic i64, ptr @counter monotonic, align 8
  ret i64 %result
}

; OBJVIEW-LABEL: "name": "cyclic_write_barrier"
; OBJVIEW: "kind": "unsafe_point"
; OBJVIEW: "value": -2
; OBJVIEW-NOT: "kind":
; OBJVIEW: "value": -1
