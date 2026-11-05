target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @consume(ptr, i64)
declare goabiinternal i1 @write(ptr)
declare goabiinternal ptr @advance(ptr)
declare goabiinternal void @replace(ptr)
declare goabiinternal ptr @fresh()

; The base of a chunk address and the passive writer must be homed at the
; outer entry. Rebuilding the chunk from its home at the load ends its SSA
; lifetime before the indirect call. No extra root is needed to reinitialize
; an inner home on the next outer iteration.
; IR-LABEL: define goabiinternal void @nested_counted(
; IR-DAG: %data.statepoint.home = alloca ptr{{.*}}!llvm.stackcoloring.no_merge
; IR-DAG: %writer.statepoint.home = alloca ptr{{.*}}!llvm.stackcoloring.no_merge
; IR-DAG: store ptr %data, ptr %data.statepoint.home
; IR-DAG: store ptr %writer, ptr %writer.statepoint.home
; IR: outer:
; IR-NOT: store ptr
; IR: body:
; IR-NOT: store ptr
; IR: %[[BASE:.*]] = load volatile ptr, ptr %data.statepoint.home
; IR: getelementptr i64, ptr %[[BASE]], i64 %offset
; IR: @llvm.experimental.gc.statepoint
; IR-SAME: "gc-live"(ptr %data.statepoint.home, ptr %writer.statepoint.home)
; IR-NOT: @llvm.experimental.gc.relocate
; IR-NOT: store ptr
; IR: flush:
; IR: load volatile ptr, ptr %writer.statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR-SAME: "gc-live"(ptr %data.statepoint.home, ptr %writer.statepoint.home)
; IR-NOT: store ptr
; IR: ret void

; Production LSR creates a loop-carried derived PHI for this reduced case.
; That representation conservatively retains its base spill. The passive writer
; still has one authoritative home and must not be rewritten on the backedge.
; X86-LABEL: name:            nested_counted
; X86: LIFETIME_START %fixed-stack.[[W:[0-9]+]]
; X86: MOV64mr %fixed-stack.[[W]]
; X86: bb.{{[0-9]+}}.body:
; X86-NOT: MOV64mr %fixed-stack.[[W]]
; X86: MOV64mr %stack.[[D:[0-9]+]],
; X86: STATEPOINT {{.*}}%fixed-stack.[[W]]
; X86-NOT: MOV64mr %fixed-stack.[[W]]
; X86: MOV64rm %stack.[[D]],
; X86: bb.{{[0-9]+}}.flush:
; X86: MOV64rm %fixed-stack.[[W]]{{.*}}volatile
;
; A64-LABEL: name:            nested_counted
; A64: LIFETIME_START %fixed-stack.[[W:[0-9]+]]
; A64: STRXui {{.*}}%fixed-stack.[[W]]
; A64: bb.{{[0-9]+}}.body:
; A64-NOT: STRXui {{.*}}%fixed-stack.[[W]]
; A64: STRXui {{.*}}%stack.[[D:[0-9]+]],
; A64: STATEPOINT {{.*}}%fixed-stack.[[W]]
; A64-NOT: STRXui {{.*}}%fixed-stack.[[W]]
; A64: LDRXui %stack.[[D]],
; A64: bb.{{[0-9]+}}.flush:
; A64: LDRXui %fixed-stack.[[W]]{{.*}}volatile
define goabiinternal void @nested_counted(ptr %writer, ptr %data, i64 %method, i64 %n) gc "goallc" {
entry:
  br label %outer
outer:
  %offset = phi i64 [0, %entry], [%next.chunk, %flush]
  %more = icmp ult i64 %offset, %n
  br i1 %more, label %chunk, label %exit
chunk:
  %remaining = sub i64 %n, %offset
  %short = icmp ult i64 %remaining, 4
  %limit = select i1 %short, i64 %remaining, i64 4
  %base = getelementptr i64, ptr %data, i64 %offset
  br label %header
header:
  %i = phi i64 [0, %chunk], [%next, %body]
  %in.chunk = icmp ult i64 %i, %limit
  br i1 %in.chunk, label %body, label %flush
body:
  %element = getelementptr i64, ptr %base, i64 %i
  %value = load i64, ptr %element
  %address = inttoptr i64 %method to ptr
  %fn = load ptr, ptr %address
  %bytes = shl i64 %i, 3
  call goabiinternal void %fn(i64 %value, i64 %bytes, i64 %offset)
  %next = add i64 %i, 1
  br label %header
flush:
  %failed = call goabiinternal i1 @write(ptr %writer)
  %next.chunk = add i64 %offset, 4
  br i1 %failed, label %exit, label %outer
exit:
  ret void
}

; A new pointer each iteration is not invariant; preserve the relocation chain.
; IR-LABEL: define goabiinternal ptr @changing(
; IR-NOT: statepoint.home
; IR: @llvm.experimental.gc.relocate
; IR: ret ptr
define goabiinternal ptr @changing(ptr %first, i64 %n) gc "goallc" {
entry:
  br label %loop
loop:
  %p = phi ptr [%first, %entry], [%new, %loop]
  %i = phi i64 [0, %entry], [%next, %loop]
  %new = call goabiinternal ptr @advance(ptr %p)
  %next = add i64 %i, 1
  %more = icmp ult i64 %next, %n
  br i1 %more, label %loop, label %exit
exit:
  ret ptr %p
}

; Alias mutation changes the loaded pointer, even with an invariant slot address.
; IR-LABEL: define goabiinternal i64 @aliased(
; IR-NOT: %p.statepoint.home
; IR: @llvm.experimental.gc.relocate
; IR-NOT: %p.statepoint.home
; IR: ret i64
define goabiinternal i64 @aliased(ptr %slot, i64 %n) gc "goallc" {
entry:
  br label %loop
loop:
  %i = phi i64 [0, %entry], [%next, %loop]
  %p = load ptr, ptr %slot
  call goabiinternal void @replace(ptr %slot)
  %value = load i64, ptr %p
  %next = add i64 %i, 1
  %more = icmp ult i64 %next, %n
  br i1 %more, label %loop, label %exit
exit:
  ret i64 %value
}

; A bypass can execute arbitrarily many iterations without a call. A static
; single-call count alone must not authorize a home in this shape.
; IR-LABEL: define goabiinternal ptr @conditional_call(
; IR-NOT: statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR-NOT: statepoint.home
; IR: ret ptr
define goabiinternal ptr @conditional_call(ptr %p, i64 %n, i1 %run) gc "goallc" {
entry:
  br label %header
header:
  %i = phi i64 [0, %entry], [%next, %latch]
  br i1 %run, label %call, label %latch
call:
  call goabiinternal void @consume(ptr %p, i64 %i)
  br label %latch
latch:
  %next = add i64 %i, 1
  %more = icmp ult i64 %next, %n
  br i1 %more, label %header, label %exit
exit:
  ret ptr %p
}

; Reloads in a numeric inner loop would be more frequent than the outer calls.
; IR-LABEL: define goabiinternal i64 @numeric_inner(
; IR-NOT: statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR-NOT: statepoint.home
; IR: ret i64
define goabiinternal i64 @numeric_inner(ptr %p, i64 %n) gc "goallc" {
entry:
  br label %outer
outer:
  %j = phi i64 [0, %entry], [%next.outer, %flush]
  br label %inner
inner:
  %i = phi i64 [0, %outer], [%next, %inner]
  %addr = getelementptr i64, ptr %p, i64 %i
  %value = load volatile i64, ptr %addr
  %next = add i64 %i, 1
  %more = icmp ult i64 %next, %n
  br i1 %more, label %inner, label %flush
flush:
  call goabiinternal void @consume(ptr %p, i64 %value)
  call goabiinternal void @consume(ptr %p, i64 %value)
  call goabiinternal void @consume(ptr null, i64 %value)
  %next.outer = add i64 %j, 1
  %again = icmp ult i64 %next.outer, %n
  br i1 %again, label %outer, label %exit
exit:
  ret i64 %value
}

; Re-entering with a fresh pointer requires a new store on each outer iteration.
; IR-LABEL: define goabiinternal void @fresh_outer(
; IR: outer:
; IR: @llvm.experimental.gc.statepoint
; IR: store ptr {{.*}}, ptr %p.statepoint.home
; IR: inner:
; IR-NOT: store ptr
; IR: load volatile ptr, ptr %p.statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR-NOT: store ptr
; IR: ret void
define goabiinternal void @fresh_outer(i64 %n) gc "goallc" {
entry:
  br label %outer
outer:
  %j = phi i64 [0, %entry], [%next.outer, %latch]
  %p = call goabiinternal ptr @fresh()
  br label %inner
inner:
  %i = phi i64 [0, %outer], [%next, %inner]
  call goabiinternal void @consume(ptr %p, i64 %i)
  %next = add i64 %i, 1
  %more = icmp ult i64 %next, %n
  br i1 %more, label %inner, label %latch
latch:
  %next.outer = add i64 %j, 1
  %again = icmp ult i64 %next.outer, %n
  br i1 %again, label %outer, label %exit
exit:
  ret void
}

; The original base and an inserted aggregate leaf describe the same pointer.
; Preserve their existing relocation representation rather than introducing
; independent homes for aliases of the same object.
; IR-LABEL: define goabiinternal i64 @inserted_alias(
; IR-NOT: statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR-NOT: statepoint.home
; IR: ret i64
define goabiinternal i64 @inserted_alias(ptr %p, i64 %n) gc "goallc" {
entry:
  %pair = insertvalue {ptr, i64} poison, ptr %p, 0
  br label %loop
loop:
  %i = phi i64 [0, %entry], [%next, %loop]
  call goabiinternal void @consume(ptr %p, i64 %i)
  %next = add i64 %i, 1
  %more = icmp ult i64 %next, %n
  br i1 %more, label %loop, label %exit
exit:
  %alias = extractvalue {ptr, i64} %pair, 0
  %value = load i64, ptr %alias
  ret i64 %value
}

; A derived address crossing a merge outside the initialized home region must
; keep its base relocation, including when another path bypasses the loop.
; IR-LABEL: define goabiinternal ptr @bypassed_derived(
; IR-NOT: statepoint.home
; IR: @llvm.experimental.gc.relocate
; IR-NOT: statepoint.home
; IR: ret ptr
define goabiinternal ptr @bypassed_derived(ptr %p, i1 %enter, i1 %again) gc "goallc" {
entry:
  %derived = getelementptr i8, ptr %p, i64 3
  br i1 %enter, label %loop, label %exit
loop:
  call goabiinternal void @consume(ptr %p, i64 0)
  br i1 %again, label %loop, label %exit
exit:
  ret ptr %derived
}

; The selected shape requires one latch, even when a call dominates both.
; IR-LABEL: define goabiinternal ptr @multiple_latches(
; IR-NOT: statepoint.home
; IR: @llvm.experimental.gc.relocate
; IR-NOT: statepoint.home
; IR: ret ptr
define goabiinternal ptr @multiple_latches(ptr %p, i1 %which, i1 %again) gc "goallc" {
entry:
  br label %header
header:
  call goabiinternal void @consume(ptr %p, i64 0)
  br i1 %which, label %left, label %right
left:
  br i1 %again, label %header, label %exit
right:
  br i1 %again, label %header, label %exit
exit:
  ret ptr %p
}

; Reconstructing an address for a captured pointer carrier is deliberately
; outside this optimization. The final statepoint repair remains authoritative.
; IR-LABEL: define goabiinternal void @captured_derived(
; IR-NOT: %p.statepoint.home
; IR: @llvm.experimental.gc.relocate
; IR-NOT: %p.statepoint.home
; IR: ret void
define goabiinternal void @captured_derived(ptr %p, ptr %slot, i64 %n) gc "goallc" {
entry:
  %derived = getelementptr i64, ptr %p, i64 2
  br label %loop
loop:
  %i = phi i64 [0, %entry], [%next, %loop]
  call goabiinternal void @consume(ptr %p, i64 %i)
  store ptr %derived, ptr %slot
  %next = add i64 %i, 1
  %more = icmp ult i64 %next, %n
  br i1 %more, label %loop, label %exit
exit:
  ret void
}
