target triple = "x86_64-unknown-linux-goobj"

%slice = type { ptr, i64, i64 }

declare goabiinternal void @suspend()
declare goabiinternal i64 @result()
declare goabi0 void @"produce<ABI0>"(ptr goret(%slice) align 8 "goretindex"="0")
declare void @llvm.lifetime.start.p0(ptr captures(none))

; Calls without token users or live pointers need no continuation boundary.
; Keep both safepoints and the original branch in the same block.
; IR-LABEL: define goabiinternal i64 @empty_continuations(
; IR: entry:
; IR-NEXT: %statepoint_token{{[0-9]*}} = call {{.*}}@llvm.experimental.gc.statepoint
; IR-NEXT: %statepoint_token{{[0-9]*}} = call {{.*}}@llvm.experimental.gc.statepoint
; IR-NEXT: br i1 %choose, label %left, label %right
; IR-NOT: statepoint.cont
; IR: left:
; IR-NEXT: ret i64 %value
; IR: right:
; IR-NEXT: ret i64 0
define goabiinternal i64 @empty_continuations(i64 %value, i1 %choose)
    gc "goallc" {
entry:
  call goabiinternal void @suspend()
  call goabiinternal void @suspend()
  br i1 %choose, label %left, label %right
left:
  ret i64 %value
right:
  ret i64 0
}

; A scalar result requires a fresh SelectionDAG scope even without gc-live.
; IR-LABEL: define goabiinternal i64 @scalar_result()
; IR: @llvm.experimental.gc.statepoint
; IR-NEXT: br label %entry.statepoint.cont
; IR: entry.statepoint.cont:
; IR-NEXT: [[RESULT:%[^ ]+]] = call i64 @llvm.experimental.gc.result
; IR-NEXT: ret i64 [[RESULT]]
define goabiinternal i64 @scalar_result() gc "goallc" {
entry:
  %value = call goabiinternal i64 @result()
  ret i64 %value
}

; Relocated heap pointers still require the continuation boundary.
; IR-LABEL: define goabiinternal i64 @heap_pointer(
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %pointer)
; IR-NEXT: br label %entry.statepoint.cont
; IR: entry.statepoint.cont:
; IR-NEXT: [[RELOC:%[^ ]+]] = call {{.*}}ptr @llvm.experimental.gc.relocate
; IR-NEXT: %value = load i64, ptr [[RELOC]]
define goabiinternal i64 @heap_pointer(ptr %pointer) gc "goallc" {
entry:
  call goabiinternal void @suspend()
  %value = load i64, ptr %pointer
  ret i64 %value
}

; Fixed frame roots have no gc.relocate, but must still keep the boundary.
; IR-LABEL: define goabiinternal i64 @fixed_frame_pointer(
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %slot)
; IR-NEXT: br label %entry.statepoint.cont
; IR: entry.statepoint.cont:
; IR-NEXT: %value = load i64, ptr %slot
define goabiinternal i64 @fixed_frame_pointer(i64 %input) gc "goallc" {
entry:
  %slot = alloca i64, align 8
  store i64 %input, ptr %slot
  call goabiinternal void @suspend()
  %value = load i64, ptr %slot
  ret i64 %value
}

; A goret call defines its result carrier, so its pre-call contents are not
; gc-live. Its live fixed-frame address independently requires the boundary.
; IR-LABEL: define goabiinternal i64 @goret_carrier()
; IR: call goabi0 token {{.*}}@llvm.experimental.gc.statepoint
; IR-NOT: "gc-live"
; IR-NEXT: br label %entry.statepoint.cont
; IR: entry.statepoint.cont:
; IR: load i64
define goabiinternal i64 @goret_carrier() gc "goallc" {
entry:
  %slot = alloca %slice, align 8
  call void @llvm.lifetime.start.p0(ptr %slot)
  call goabi0 void @"produce<ABI0>"(ptr goret(%slice) align 8 "goretindex"="0" %slot)
  %length.address = getelementptr inbounds %slice, ptr %slot, i32 0, i32 1
  %length = load i64, ptr %length.address, align 8
  ret i64 %length
}

; An empty safepoint does not introduce a new backedge predecessor.
; IR-LABEL: define goabiinternal i64 @empty_loop(
; IR: %index = phi i64 [ 0, %entry ], [ %next, %loop ]
; IR-NEXT: %statepoint_token{{[0-9]*}} = call {{.*}}@llvm.experimental.gc.statepoint
; IR-NEXT: %next = add i64 %index, 1
; IR-NEXT: %again = icmp ult i64 %next, %limit
; IR-NEXT: br i1 %again, label %loop, label %exit
define goabiinternal i64 @empty_loop(i64 %limit) gc "goallc" {
entry:
  br label %loop
loop:
  %index = phi i64 [ 0, %entry ], [ %next, %loop ]
  call goabiinternal void @suspend()
  %next = add i64 %index, 1
  %again = icmp ult i64 %next, %limit
  br i1 %again, label %loop, label %exit
exit:
  ret i64 %next
}
