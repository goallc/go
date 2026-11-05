target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal i1 @next(ptr)
declare goabiinternal ptr @make_iterator()

; IR-LABEL: define goabiinternal ptr @iterator_loop(
; IR: %iter.statepoint.home = alloca ptr{{.*}}!llvm.stackcoloring.no_merge
; IR: store ptr %{{.*}}, ptr %b.statepoint.home
; IR: store ptr %{{.*}}, ptr %iter.statepoint.home
; IR: loop:
; IR-NOT: store ptr
; IR: load volatile ptr, ptr %iter.statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR-SAME: "gc-live"(ptr %iter.statepoint.home, ptr %b.statepoint.home)
; IR-NOT: store ptr
; IR: br i1
; IR: exit:
; IR: ret ptr


; MIR-LABEL: name:            iterator_loop
; MIR: bb.1.entry.statepoint.cont:
; MIR: MOV64mr %stack.1.b.statepoint.home
; MIR: MOV64mr %stack.0.iter.statepoint.home
; MIR: bb.2.loop:
; MIR-NOT: MOV64mr
; MIR: MOV64rm %stack.0.iter.statepoint.home{{.*}}volatile
; MIR-NOT: MOV64mr
; MIR: STATEPOINT {{.*}} @next, {{.*}}%stack.0.iter.statepoint.home{{.*}}%stack.1.b.statepoint.home
; MIR-NOT: MOV64mr
; MIR: JCC_1 %bb.2
; MIR-NOT: MOV64mr
; MIR: bb.4.exit:
; MIR: MOV64rm %stack.1.b.statepoint.home{{.*}}volatile

; A64-LABEL: name:            iterator_loop
; A64: bb.1.entry.statepoint.cont:
; A64: STRXui {{.*}}%stack.1.b.statepoint.home
; A64: STRXui {{.*}}%stack.0.iter.statepoint.home
; A64: bb.2.loop:
; A64-NOT: STRXui
; A64: LDRXui %stack.0.iter.statepoint.home{{.*}}volatile
; A64-NOT: STRXui
; A64: STATEPOINT {{.*}} @next, {{.*}}%stack.0.iter.statepoint.home{{.*}}%stack.1.b.statepoint.home
; A64-NOT: STRXui
; A64: TBNZW {{.*}}%bb.2
; A64-NOT: STRXui
; A64: bb.4.exit:
; A64: LDRXui %stack.1.b.statepoint.home{{.*}}volatile

define goabiinternal ptr @iterator_loop(ptr %b) gc "goallc" {
entry:
  %iter = call goabiinternal ptr @make_iterator()
  br label %loop
loop:
  %more = call goabiinternal i1 @next(ptr %iter)
  br i1 %more, label %loop, label %exit
exit:
  ret ptr %b
}

; A new pointer selected on the backedge is not an invariant home candidate.
; IR-LABEL: define goabiinternal ptr @changing_pointer(
; IR-NOT: statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR: ret ptr

; MIR-LABEL: name:            changing_pointer
; MIR: bb.{{[0-9]+}}.loop:
; MIR: MOV64mr
; MIR: STATEPOINT {{.*}} @advance,
; A64-LABEL: name:            changing_pointer
; A64: bb.{{[0-9]+}}.loop:
; A64: STRXui
; A64: STATEPOINT {{.*}} @advance,

define goabiinternal ptr @changing_pointer(ptr %first) gc "goallc" {
entry:
  br label %loop
loop:
  %p = phi ptr [ %first, %entry ], [ %new, %loop ]
  %new = call goabiinternal ptr @advance(ptr %p)
  %done = icmp eq ptr %new, null
  br i1 %done, label %exit, label %loop
exit:
  ret ptr %p
}
declare goabiinternal ptr @advance(ptr)

; A reload from aliased memory is a new value, even when the address is invariant.
; IR-LABEL: define goabiinternal ptr @alias_write(
; IR-NOT: %p.statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR: ret ptr

; MIR-LABEL: name:            alias_write
; MIR: bb.{{[0-9]+}}.loop:
; MIR: MOV64mr
; MIR: STATEPOINT {{.*}} @mutate,
; A64-LABEL: name:            alias_write
; A64: bb.{{[0-9]+}}.loop:
; A64: STRXui
; A64: STATEPOINT {{.*}} @mutate,

define goabiinternal ptr @alias_write(ptr %address) gc "goallc" {
entry:
  br label %loop
loop:
  %p = load ptr, ptr %address
  %more = call goabiinternal i1 @mutate(ptr %address, ptr %p)
  br i1 %more, label %loop, label %exit
exit:
  ret ptr %p
}
declare goabiinternal i1 @mutate(ptr, ptr)

; Two incoming entry edges cannot share an unconditionally initialized home.
; IR-LABEL: define goabiinternal ptr @multiple_entries(
; IR-NOT: statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR: ret ptr

; MIR-LABEL: name:            multiple_entries
; MIR: bb.{{[0-9]+}}.loop:
; MIR: MOV64mr
; MIR: STATEPOINT {{.*}} @next,
; A64-LABEL: name:            multiple_entries
; A64: bb.{{[0-9]+}}.loop:
; A64: STRXui
; A64: STATEPOINT {{.*}} @next,

define goabiinternal ptr @multiple_entries(ptr %a, ptr %b, i1 %which) gc "goallc" {
entry:
  br i1 %which, label %left, label %right
left:
  br label %loop
right:
  br label %loop
loop:
  %p = phi ptr [ %a, %left ], [ %b, %right ], [ %p, %loop ]
  %more = call goabiinternal i1 @next(ptr %p)
  br i1 %more, label %loop, label %exit
exit:
  ret ptr %p
}

; Two safepoints share the home, but each material use after a call must
; reload its current value. The exit reload is outside the repeated cycle.
; IR-LABEL: define goabiinternal ptr @two_safepoints(
; IR: store ptr %p, ptr %p.statepoint.home
; IR: loop:
; IR-NOT: store ptr
; IR: load volatile ptr, ptr %p.statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR-SAME: "gc-live"(ptr %p.statepoint.home)
; IR-NOT: @llvm.experimental.gc.relocate
; IR: load volatile ptr, ptr %p.statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR-SAME: "gc-live"(ptr %p.statepoint.home)
; IR-NOT: store ptr
; IR-NOT: @llvm.experimental.gc.relocate
; IR: exit:
; IR: %[[P:.*]] = load volatile ptr, ptr %p.statepoint.home
; IR: ret ptr %[[P]]

define goabiinternal ptr @two_safepoints(ptr %p) gc "goallc" {
entry:
  br label %loop
loop:
  %ignored = call goabiinternal i1 @next(ptr %p)
  %more = call goabiinternal i1 @next(ptr %p)
  br i1 %more, label %loop, label %exit
exit:
  ret ptr %p
}

; Independent inner loops must initialize each newly returned iterator.
; IR-LABEL: define goabiinternal void @reinitialize(
; IR: %iter.statepoint.home = alloca ptr
; IR: outer:
; IR: @llvm.experimental.gc.statepoint
; IR: store ptr %{{.*}}, ptr %iter.statepoint.home
; IR: inner:
; IR-NOT: store ptr
; IR: load volatile ptr, ptr %iter.statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR-NOT: store ptr
; IR: br i1
; IR: ret void

define goabiinternal void @reinitialize(i64 %count) gc "goallc" {
entry:
  br label %outer
outer:
  %i = phi i64 [ 0, %entry ], [ %inc, %latch ]
  %iter = call goabiinternal ptr @make_iterator()
  br label %inner
inner:
  %more = call goabiinternal i1 @next(ptr %iter)
  br i1 %more, label %inner, label %latch
latch:
  %inc = add i64 %i, 1
  %again = icmp ult i64 %inc, %count
  br i1 %again, label %outer, label %exit
exit:
  ret void
}

; The branch still uses next's result, but the receiver changes each time.
; IR-LABEL: define goabiinternal ptr @changed_receiver(
; IR-NOT: statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR: @llvm.experimental.gc.relocate
; IR: ret ptr

define goabiinternal ptr @changed_receiver(ptr %first) gc "goallc" {
entry:
  br label %loop
loop:
  %p = phi ptr [ %first, %entry ], [ %new, %loop ]
  %more = call goabiinternal i1 @next(ptr %p)
  %new = load ptr, ptr %p
  br i1 %more, label %loop, label %exit
exit:
  ret ptr %new
}

; Entry through a conditional predecessor needs its own initialization edge.
; IR-LABEL: define goabiinternal void @conditional_entry(
; IR: entry:
; IR: %p.statepoint.home = alloca ptr
; IR-NOT: store ptr
; IR: br i1 %enter, label %loop.statepoint.preheader, label %exit
; IR: loop.statepoint.preheader:
; IR: store ptr %p, ptr %p.statepoint.home
; IR: loop:
; IR-NOT: store ptr
; IR: load volatile ptr, ptr %p.statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR-NOT: store ptr
; IR: br i1
; IR: ret void

define goabiinternal void @conditional_entry(ptr %p, i1 %enter) gc "goallc" {
entry:
  br i1 %enter, label %loop, label %exit
loop:
  %more = call goabiinternal i1 @next(ptr %p)
  br i1 %more, label %loop, label %exit
exit:
  ret void
}

; More material use regions than preserved calls do not amortize a home.
; IR-LABEL: define goabiinternal i64 @extra_reload(
; IR-NOT: statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR: @llvm.experimental.gc.relocate
; IR: ret i64

define goabiinternal i64 @extra_reload(ptr %p) gc "goallc" {
entry:
  br label %loop
loop:
  %more = call goabiinternal i1 @next(ptr %p)
  %value = load i64, ptr %p
  br i1 %more, label %loop, label %exit
exit:
  ret i64 %value
}

; A passive outer-loop invariant must not keep a second SSA root alive to
; initialize an inner-loop home on the next outer iteration.
; IR-LABEL: define goabiinternal void @outer_invariant(ptr %b)
; IR: %b.statepoint.home = alloca ptr
; IR: store ptr %b, ptr %b.statepoint.home
; IR: outer:
; IR: @llvm.experimental.gc.statepoint
; IR: store ptr %{{.*}}, ptr %iter.statepoint.home
; IR: inner:
; IR-NOT: phi ptr
; IR: load volatile ptr, ptr %iter.statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR-SAME: "gc-live"(ptr %iter.statepoint.home, ptr %b.statepoint.home)
; IR-NOT: @llvm.experimental.gc.relocate
; IR: latch:
; IR: load volatile ptr, ptr %b.statepoint.home
; IR: ret void

; MIR-LABEL: name:            outer_invariant
; MIR: bb.[[INNER:[0-9]+]].inner:
; MIR-NOT: MOV64mr
; MIR: MOV64rm %stack.{{[0-9]+}}.iter.statepoint.home{{.*}}volatile
; MIR-NOT: MOV64mr
; MIR: STATEPOINT {{.*}} @next,
; MIR-NOT: MOV64mr
; MIR: JCC_1 %bb.[[INNER]]
; MIR-NOT: MOV64mr
; MIR: bb.{{[0-9]+}}.latch:

; A64-LABEL: name:            outer_invariant
; A64: bb.[[INNER:[0-9]+]].inner:
; A64-NOT: STRXui
; A64: LDRXui %stack.{{[0-9]+}}.iter.statepoint.home{{.*}}volatile
; A64-NOT: STRXui
; A64: STATEPOINT {{.*}} @next,
; A64-NOT: STRXui
; A64: TBNZW {{.*}}%bb.[[INNER]]
; A64-NOT: STRXui
; A64: bb.{{[0-9]+}}.latch:

define goabiinternal void @outer_invariant(ptr %b) gc "goallc" {
entry:
  br label %outer
outer:
  %i = phi i64 [ 0, %entry ], [ %inc, %latch ]
  %iter = call goabiinternal ptr @make_iterator()
  br label %inner
inner:
  %more = call goabiinternal i1 @next(ptr %iter)
  br i1 %more, label %inner, label %latch
latch:
  %limit = load i64, ptr %b
  %inc = add i64 %i, 1
  %again = icmp ult i64 %inc, %limit
  br i1 %again, label %outer, label %exit
exit:
  ret void
}
