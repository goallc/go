target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @consume(ptr)
declare goabiinternal void @tick()
declare goabiinternal ptr @make_pointer()

; Two active safepoints can amortize a home without a passive-use heuristic.
; Each post-call material use must still read the updated home.
; IR-LABEL: define goabiinternal i64 @hot_two_calls(
; IR: %p.statepoint.home = alloca ptr{{.*}}!llvm.stackcoloring.no_merge
; IR: store ptr %p, ptr %p.statepoint.home
; IR: loop:
; IR-NOT: store ptr
; IR: load volatile ptr, ptr %p.statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR-SAME: "gc-live"(ptr %p.statepoint.home)
; IR: load volatile ptr, ptr %p.statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR-SAME: "gc-live"(ptr %p.statepoint.home)
; IR: %[[P:.*]] = load volatile ptr, ptr %p.statepoint.home
; IR: load i64, ptr %[[P]]
; IR-NOT: @llvm.experimental.gc.relocate
; IR-NOT: store ptr
; IR: ret i64
;
; X86-LABEL: name:            hot_two_calls
; X86: LIFETIME_START [[HOME:%[^ ,]+]]
; X86: MOV64mr [[HOME]],
; X86: bb.{{[0-9]+}}.loop:
; X86-NOT: MOV64mr [[HOME]],
; X86: STATEPOINT {{.*}} [[HOME]], 0,
; X86-NOT: MOV64mr [[HOME]],
; X86: STATEPOINT {{.*}} [[HOME]], 0,
; X86-NOT: MOV64mr [[HOME]],
; X86: RET
;
; A64-LABEL: name:            hot_two_calls
; A64: LIFETIME_START [[HOME:%[^ ,]+]]
; A64: STRXui {{.*}}, [[HOME]],
; A64: bb.{{[0-9]+}}.loop:
; A64-NOT: STRXui {{.*}}, [[HOME]],
; A64: STATEPOINT {{.*}} [[HOME]], 0,
; A64-NOT: STRXui {{.*}}, [[HOME]],
; A64: STATEPOINT {{.*}} [[HOME]], 0,
; A64-NOT: STRXui {{.*}}, [[HOME]],
; A64: RET

define goabiinternal i64 @hot_two_calls(ptr %p, i64 %n) gc "goallc" {
entry:
  br label %loop
loop:
  %i = phi i64 [0, %entry], [%next, %loop]
  call goabiinternal void @consume(ptr %p)
  call goabiinternal void @consume(ptr %p)
  %value = load i64, ptr %p
  %next = add i64 %i, 1
  %more = icmp ult i64 %next, %n
  br i1 %more, label %loop, label %exit
exit:
  ret i64 %value
}

; No individual call dominates every backedge. Both call paths share the home;
; the exit-only reload is charged once, not once per loop iteration.
; IR-LABEL: define goabiinternal ptr @alternative_calls(
; IR: store ptr %p, ptr %p.statepoint.home
; IR: left:
; IR-NOT: store ptr
; IR: load volatile ptr, ptr %p.statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR-SAME: "gc-live"(ptr %p.statepoint.home)
; IR: right:
; IR-NOT: store ptr
; IR: load volatile ptr, ptr %p.statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR-SAME: "gc-live"(ptr %p.statepoint.home)
; IR: exit:
; IR: %[[END:.*]] = load volatile ptr, ptr %p.statepoint.home
; IR: ret ptr %[[END]]
define goabiinternal ptr @alternative_calls(ptr %p, i1 %which, i1 %again) gc "goallc" {
entry:
  br label %header
header:
  br i1 %which, label %left, label %right
left:
  call goabiinternal void @consume(ptr %p)
  br i1 %again, label %header, label %exit
right:
  call goabiinternal void @consume(ptr %p)
  br i1 %again, label %header, label %exit
exit:
  ret ptr %p
}

; A many-call cold loop still perturbs the frame on the hot bypass path.
; Static call count alone is not evidence that the frame cost is amortized.
; IR-LABEL: define goabiinternal i64 @cold_three_calls(
; IR-NOT: statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR: @llvm.experimental.gc.relocate
; IR: @llvm.experimental.gc.statepoint
; IR: @llvm.experimental.gc.relocate
; IR: @llvm.experimental.gc.statepoint
; IR: @llvm.experimental.gc.relocate
; IR-NOT: statepoint.home
; IR: ret i64
define goabiinternal i64 @cold_three_calls(ptr %p, i1 %cold, i1 %again) gc "goallc" {
entry:
  br i1 %cold, label %loop, label %bypass, !prof !0
loop:
  call goabiinternal void @tick()
  call goabiinternal void @tick()
  call goabiinternal void @tick()
  %value = load i64, ptr %p
  br i1 %again, label %loop, label %exit
exit:
  ret i64 %value
bypass:
  ret i64 0
}

; Frequent ordinary uses and a rare safepoint do not justify a volatile load
; on every iteration. This conditional-call rejection is a cost decision.
; IR-LABEL: define goabiinternal i64 @rare_call_hot_uses(
; IR-NOT: statepoint.home
; IR: load i64, ptr
; IR: @llvm.experimental.gc.statepoint
; IR: @llvm.experimental.gc.relocate
; IR-NOT: statepoint.home
; IR: ret i64
define goabiinternal i64 @rare_call_hot_uses(ptr %p, i1 %rare, i1 %again) gc "goallc" {
entry:
  br label %header
header:
  %value = load i64, ptr %p
  br i1 %rare, label %call, label %latch, !prof !1
call:
  call goabiinternal void @tick()
  br label %latch
latch:
  br i1 %again, label %header, label %exit
exit:
  ret i64 %value
}

; The outer numeric loop rejects a home. An inner-only home would still need
; the original p on every re-entry, so it must not claim to eliminate that root.
; IR-LABEL: define goabiinternal i64 @surviving_reentry_root(
; IR-NOT: statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR: @llvm.experimental.gc.relocate
; IR-NOT: statepoint.home
; IR: ret i64
define goabiinternal i64 @surviving_reentry_root(ptr %p, i64 %n, i1 %again) gc "goallc" {
entry:
  br label %outer
outer:
  br label %numeric
numeric:
  %i = phi i64 [0, %outer], [%next, %numeric]
  %addr = getelementptr i64, ptr %p, i64 %i
  %value = load volatile i64, ptr %addr
  %next = add i64 %i, 1
  %more = icmp ult i64 %next, %n
  br i1 %more, label %numeric, label %inner.preheader
inner.preheader:
  br label %inner
inner:
  call goabiinternal void @consume(ptr %p)
  br i1 %again, label %inner, label %latch
latch:
  br i1 %again, label %outer, label %exit
exit:
  ret i64 %value
}

; A derived pointer PHI changes the recipe/identity across iterations. It
; retains the required base relocation instead of being mistaken for a pure
; GEP chain that ends at a load.
; IR-LABEL: define goabiinternal i64 @derived_carrier(
; IR-NOT: %p.statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR: @llvm.experimental.gc.relocate
; IR-NOT: %p.statepoint.home
; IR: ret i64
define goabiinternal i64 @derived_carrier(ptr %p, i64 %n) gc "goallc" {
entry:
  %first = getelementptr i64, ptr %p, i64 1
  br label %loop
loop:
  %current = phi ptr [%first, %entry], [%next.ptr, %loop]
  %i = phi i64 [0, %entry], [%next, %loop]
  call goabiinternal void @consume(ptr %p)
  %value = load i64, ptr %current
  %next.ptr = getelementptr i64, ptr %current, i64 1
  %next = add i64 %i, 1
  %more = icmp ult i64 %next, %n
  br i1 %more, label %loop, label %exit
exit:
  ret i64 %value
}

; A common exit also reachable without entering the loop still reads the
; original SSA root. Until an exit merge can be homed, do not add a second root.
; IR-LABEL: define goabiinternal i64 @common_bypass_exit(
; IR-NOT: statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR: @llvm.experimental.gc.relocate
; IR-NOT: statepoint.home
; IR: ret i64
define goabiinternal i64 @common_bypass_exit(ptr %p, i1 %enter, i1 %again) gc "goallc" {
entry:
  br i1 %enter, label %loop, label %exit
loop:
  call goabiinternal void @consume(ptr %p)
  br i1 %again, label %loop, label %exit
exit:
  %value = load i64, ptr %p
  ret i64 %value
}

!0 = !{!"branch_weights", i32 1, i32 1000000}
!1 = !{!"branch_weights", i32 1, i32 1023}
