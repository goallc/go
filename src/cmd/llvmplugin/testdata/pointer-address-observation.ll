target triple = "aarch64-apple-darwin-goobj"

; O2-LABEL: define goabiinternal i1 @observe(
; O2-COUNT-2: call i64 @llvm.go.pointer.address.i64.p0
; O2-NOT: ret i1 true

; O2-LABEL: define goabiinternal ptr @restore(
; O2: %pointer = {{(tail )?}}call ptr @llvm.go.pointer.from.address.p0.i64(i64 %address)
; O2-NEXT: {{(tail )?}}call goabiinternal void @callee()
; O2: ret ptr %pointer

; O2-LABEL: define goabiinternal i64 @terminal(
; O2: {{(tail )?}}call goabiinternal void @callee()
; O2-NEXT: %pointer = {{(tail )?}}call ptr @llvm.go.pointer.from.address.p0.i64(i64 %address)
; O2-NEXT: %value = load i64, ptr %pointer

; O2-LABEL: define goabiinternal i64 @terminal_direct_loop(
; O2: %pointer = inttoptr i64 %address to ptr
; O2: {{(tail )?}}call goabiinternal void @callee()
; O2: %value = load i64, ptr %pointer

; O2-LABEL: define goabiinternal i64 @terminal_direct_derived_loop(
; O2: inttoptr i64 %address to ptr{{.*}}!goallc.notinheap
; O2: {{(tail )?}}call goabiinternal void @callee()
; O2: load i64, ptr

; O2-LABEL: define goabiinternal i64 @terminal_direct_mixed(
; O2: %nih = inttoptr i64 %address to ptr{{.*}}!goallc.notinheap
; O2: %selected = select i1 %use_nih, ptr %nih, ptr %ordinary
; O2: {{(tail )?}}call goabiinternal void @callee()
; O2: load i64, ptr %selected

; REWRITE-LABEL: define goabiinternal i1 @observe(
; REWRITE-NOT: llvm.go.pointer.address
; REWRITE: %before.lowered = ptrtoint ptr %pointer to i64
; REWRITE: %pointer.relocated = call coldcc ptr @llvm.experimental.gc.relocate
; REWRITE: %after.lowered = ptrtoint ptr %pointer.relocated to i64
; REWRITE: %same = icmp eq i64 %before.lowered, %after.lowered

; REWRITE-LABEL: define goabiinternal ptr @restore(
; REWRITE-NOT: llvm.go.pointer.from.address
; REWRITE: %pointer.lowered = inttoptr i64 %address to ptr
; REWRITE: %pointer.lowered.relocated = call coldcc ptr @llvm.experimental.gc.relocate
; REWRITE: ret ptr %pointer.lowered.relocated

; REWRITE-LABEL: define goabiinternal i64 @terminal(
; REWRITE-NOT: llvm.go.pointer.from.address
; REWRITE-NOT: "gc-live"
; REWRITE: %statepoint_token = call goabiinternal token {{.*}}@llvm.experimental.gc.statepoint
; REWRITE-NOT: "gc-live"
; REWRITE: %pointer.lowered = inttoptr i64 %address to ptr
; REWRITE-NEXT: %value = load i64, ptr %pointer.lowered
; REWRITE-NOT: llvm.experimental.gc.relocate
; REWRITE: ret i64 %value

; A raw inttoptr may be hoisted by LICM, but an unmanaged address must remain
; outside GC liveness and must not be relocated across the call.
;
; REWRITE-LABEL: define goabiinternal i64 @terminal_direct_loop(
; REWRITE: %[[DIRECT_POINTER:[^ ]+]] = inttoptr i64 %address to ptr, !goallc.notinheap
; REWRITE-NOT: "gc-live"
; REWRITE: %[[DIRECT_STATEPOINT:[^ ]+]] = call goabiinternal token {{.*}}@llvm.experimental.gc.statepoint
; REWRITE-NOT: "gc-live"
; REWRITE: statepoint.cont:
; REWRITE-NEXT: %value = load i64, ptr %[[DIRECT_POINTER]]
; REWRITE-NOT: llvm.experimental.gc.relocate
; REWRITE: ret i64

; Transparent pointer derivations and loop PHIs retain NotInHeap provenance.
;
; REWRITE-LABEL: define goabiinternal i64 @terminal_direct_derived_loop(
; REWRITE-NOT: "gc-live"
; REWRITE: call goabiinternal token {{.*}}@llvm.experimental.gc.statepoint
; REWRITE-NOT: "gc-live"
; REWRITE: statepoint.cont:
; REWRITE-NOT: llvm.experimental.gc.relocate
; REWRITE: load i64, ptr
; REWRITE: ret i64

; Mixing a marked unmanaged address with an ordinary pointer must remain
; conservative: the selected value is still a relocatable GC pointer.
;
; REWRITE-LABEL: define goabiinternal i64 @terminal_direct_mixed(
; REWRITE: %selected = select i1 %use_nih, ptr %nih, ptr %ordinary
; REWRITE: call goabiinternal token {{.*}}@llvm.experimental.gc.statepoint{{.*}}[ "gc-live"(ptr %selected) ]
; REWRITE: call coldcc ptr @llvm.experimental.gc.relocate
; REWRITE: load i64, ptr
; REWRITE: ret i64

declare i64 @llvm.go.pointer.address.i64.p0(ptr)
declare ptr @llvm.go.pointer.from.address.p0.i64(i64)
declare goabiinternal void @callee()

define goabiinternal i1 @observe(ptr %pointer)
    gc "goallc" {
entry:
  %before = call i64 @llvm.go.pointer.address.i64.p0(ptr %pointer)
  call goabiinternal void @callee()
  %after = call i64 @llvm.go.pointer.address.i64.p0(ptr %pointer)
  %same = icmp eq i64 %before, %after
  ret i1 %same
}

define goabiinternal ptr @restore(i64 %address) gc "goallc" {
entry:
  %pointer = call ptr @llvm.go.pointer.from.address.p0.i64(i64 %address)
  call goabiinternal void @callee()
  ret ptr %pointer
}

define goabiinternal i64 @terminal(i64 %address) gc "goallc" {
entry:
  call goabiinternal void @callee()
  %pointer = call ptr @llvm.go.pointer.from.address.p0.i64(i64 %address)
  %value = load i64, ptr %pointer
  ret i64 %value
}

define goabiinternal i64 @terminal_direct_loop(i64 %address, i64 %count)
    gc "goallc" {
entry:
  %positive = icmp sgt i64 %count, 0
  br i1 %positive, label %loop, label %exit

loop:
  %index = phi i64 [ 0, %entry ], [ %next, %loop ]
  %sum = phi i64 [ 0, %entry ], [ %updated, %loop ]
  %pointer = inttoptr i64 %address to ptr, !goallc.notinheap !0
  call goabiinternal void @callee()
  %value = load i64, ptr %pointer, align 8
  %updated = add i64 %sum, %value
  %next = add nuw i64 %index, 1
  %more = icmp slt i64 %next, %count
  br i1 %more, label %loop, label %exit

exit:
  %result = phi i64 [ 0, %entry ], [ %updated, %loop ]
  ret i64 %result
}

define goabiinternal i64 @terminal_direct_derived_loop(i64 %address, i64 %count)
    gc "goallc" {
entry:
  %base = inttoptr i64 %address to ptr, !goallc.notinheap !0
  %positive = icmp sgt i64 %count, 0
  br i1 %positive, label %loop, label %exit

loop:
  %cursor = phi ptr [ %base, %entry ], [ %next, %loop ]
  %index = phi i64 [ 0, %entry ], [ %following, %loop ]
  %sum = phi i64 [ 0, %entry ], [ %updated, %loop ]
  call goabiinternal void @callee()
  %value = load i64, ptr %cursor, align 8
  %updated = add i64 %sum, %value
  %next = getelementptr i64, ptr %cursor, i64 1
  %following = add nuw i64 %index, 1
  %more = icmp slt i64 %following, %count
  br i1 %more, label %loop, label %exit

exit:
  %result = phi i64 [ 0, %entry ], [ %updated, %loop ]
  ret i64 %result
}

define goabiinternal i64 @terminal_direct_mixed(
    ptr %ordinary, i64 %address, i1 %use_nih) gc "goallc" {
entry:
  %nih = inttoptr i64 %address to ptr, !goallc.notinheap !0
  %selected = select i1 %use_nih, ptr %nih, ptr %ordinary
  call goabiinternal void @callee()
  %value = load i64, ptr %selected, align 8
  ret i64 %value
}

!0 = !{}
