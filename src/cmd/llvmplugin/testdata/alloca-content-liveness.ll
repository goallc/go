target triple = "x86_64-unknown-linux-goobj"

; The low bit of encoded byte size carries content liveness independently of
; gc-live address rematerialization. Inactive records remain available for the
; function-wide StackObject layout. No VarDef annotations or repeated lifetime
; starts are needed for overwrites.
; IR-LABEL: define goabiinternal ptr @overwrite(
; IR: @checkpoint{{.*}}ptr %slot, i64 8, i64 1,
; IR-LABEL: define goabiinternal ptr @overwrite_fields(
; IR: @checkpoint{{.*}}ptr %slot, i64 16, i64 3,
; IR-LABEL: define goabiinternal ptr @partial_object(
; IR: @checkpoint{{.*}}ptr %slot, i64 17, i64 3,
; IR-LABEL: define goabiinternal ptr @overlapping_move(
; IR: @checkpoint{{.*}}ptr %slot, i64 25, i64 7,
; IR-LABEL: define goabiinternal ptr @self_copy(
; IR: @checkpoint{{.*}}ptr %slot, i64 9, i64 1,
; IR-LABEL: define goabiinternal ptr @both_paths_overwrite(
; IR: @checkpoint{{.*}}ptr %slot, i64 8, i64 1,
; IR-LABEL: define goabiinternal ptr @one_path_reads(
; IR: @checkpoint{{.*}}ptr %slot, i64 9, i64 1,
; IR-LABEL: define goabiinternal void @loop_overwrite(
; IR: @checkpoint{{.*}}ptr %slot, i64 8, i64 1,
; IR-LABEL: define goabiinternal ptr @unknown_write_offset(
; IR: @checkpoint{{.*}}ptr %slot, i64 17, i64 3,
; IR-LABEL: define goabiinternal ptr @byval_self_copy(
; IR: @checkpoint{{.*}}ptr %slot, i64 9, i64 1,
; IR-LABEL: define goabiinternal ptr @mixed_alloca_phi(
; IR: @checkpoint{{.*}}ptr %left, i64 8, i64 1,{{.*}}ptr %right, i64 8, i64 1,
; IR-LABEL: define goabiinternal ptr @mixed_alloca_select(
; IR: @checkpoint{{.*}}ptr %left, i64 8, i64 1,{{.*}}ptr %right, i64 8, i64 1,
; IR-LABEL: define goabiinternal ptr @mixed_byval_select(
; IR: @checkpoint{{.*}}ptr %local, i64 8, i64 1,{{.*}}ptr %arg, i64 8, i64 1,
; IR-LABEL: define goabiinternal ptr @mixed_alloca_loop_phi(
; IR-COUNT-2: @checkpoint{{.*}}ptr %left, i64 9, i64 1,{{.*}}ptr %right, i64 9, i64 1,
; IR-LABEL: define goabiinternal ptr @loop_partial_overwrite(
; IR: @checkpoint{{.*}}ptr %slot, i64 17, i64 3,
; IR-LABEL: define goabiinternal ptr @event_free_loop_live(
; IR-COUNT-2: @checkpoint{{.*}}ptr %slot, i64 9, i64 1,
; IR-LABEL: define goabiinternal ptr @event_free_loop_killed(
; IR-COUNT-2: @checkpoint{{.*}}ptr %slot, i64 8, i64 1,
; IR-LABEL: define goabiinternal ptr @aggregate_alloca_phi(
; IR: @checkpoint{{.*}}ptr %slot, i64 9, i64 1,
; IR-LABEL: define goabiinternal ptr @aggregate_byval_select(
; IR: @checkpoint{{.*}}ptr %slot, i64 9, i64 1,

; Memory contents and loaded SSA values have different roots. Do not infer a
; unique fixed-frame origin through loads, even if a preceding store is visible.
; IR-LABEL: define goabiinternal ptr @memory_live(
; IR: @checkpoint{{.*}}ptr %box, i64 17, i64 1,{{.*}}"gc-live"(ptr %box)
; IR-LABEL: define goabiinternal ptr @loaded_heap_pointer(
; IR: @checkpoint{{.*}}ptr %box, i64 16, i64 1,{{.*}}"gc-live"(ptr %loaded.leaf.0)
; IR: %loaded.leaf.0.relocated = call coldcc ptr @llvm.experimental.gc.relocate
; IR: ret ptr %loaded.leaf.0.relocated
; IR-LABEL: define goabiinternal ptr @loaded_stack_pointer(
; IR: @checkpoint{{.*}}ptr %target, i64 8, i64 1,{{.*}}ptr %box, i64 16, i64 1,{{.*}}"gc-live"(ptr %loaded.leaf.0)
; IR: %loaded.leaf.0.relocated = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %result = load ptr, ptr %loaded.leaf.0.relocated

%pair = type { ptr, ptr }
declare goabiinternal void @observe(ptr)
declare goabiinternal void @checkpoint()
declare void @llvm.memset.inline.p0.i64(ptr, i8, i64, i1 immarg)
declare void @llvm.memmove.p0.p0.i64(ptr, ptr, i64, i1 immarg)

define goabiinternal ptr @overwrite(ptr %old) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %old, ptr %slot
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  store ptr null, ptr %slot
  %result = load ptr, ptr %slot
  ret ptr %result
}

define goabiinternal ptr @overwrite_fields(ptr %old) gc "goallc" {
entry:
  %slot = alloca %pair, align 8
  %second = getelementptr %pair, ptr %slot, i32 0, i32 1
  store ptr %old, ptr %slot
  store ptr %old, ptr %second
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  store ptr null, ptr %slot
  store ptr null, ptr %second
  %result = load ptr, ptr %second
  ret ptr %result
}

define goabiinternal ptr @partial_object(ptr %old) gc "goallc" {
entry:
  %slot = alloca %pair, align 8
  %second = getelementptr %pair, ptr %slot, i32 0, i32 1
  store ptr %old, ptr %slot
  store ptr %old, ptr %second
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  store ptr null, ptr %slot
  %result = load ptr, ptr %second
  ret ptr %result
}

define goabiinternal ptr @overlapping_move(ptr %old) gc "goallc" {
entry:
  %slot = alloca [3 x ptr], align 8
  %second = getelementptr [3 x ptr], ptr %slot, i64 0, i64 1
  %third = getelementptr [3 x ptr], ptr %slot, i64 0, i64 2
  call void @llvm.memset.inline.p0.i64(ptr %slot, i8 0, i64 24, i1 false)
  store ptr %old, ptr %slot
  store ptr %old, ptr %second
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  call void @llvm.memmove.p0.p0.i64(ptr %second, ptr %slot, i64 16, i1 false)
  %result = load ptr, ptr %third
  ret ptr %result
}

define goabiinternal ptr @self_copy(ptr %old) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %old, ptr %slot
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  call void @llvm.memmove.p0.p0.i64(ptr %slot, ptr %slot, i64 8, i1 false)
  %result = load ptr, ptr %slot
  ret ptr %result
}

define goabiinternal ptr @both_paths_overwrite(ptr %old, i1 %cond) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %old, ptr %slot
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  br i1 %cond, label %left, label %right
left:
  store ptr null, ptr %slot
  br label %merge
right:
  call void @llvm.memset.inline.p0.i64(ptr %slot, i8 0, i64 8, i1 false)
  br label %merge
merge:
  %result = load ptr, ptr %slot
  ret ptr %result
}

define goabiinternal ptr @one_path_reads(ptr %old, i1 %cond) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %old, ptr %slot
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  br i1 %cond, label %left, label %merge
left:
  store ptr null, ptr %slot
  br label %merge
merge:
  %result = load ptr, ptr %slot
  ret ptr %result
}

define goabiinternal void @loop_overwrite(ptr %old, i1 %again) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %old, ptr %slot
  br label %loop
loop:
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  store ptr null, ptr %slot
  br i1 %again, label %loop, label %exit
exit:
  ret void
}

define goabiinternal ptr @unknown_write_offset(ptr %old, i64 %index) gc "goallc" {
entry:
  %slot = alloca [2 x ptr], align 8
  call void @llvm.memset.inline.p0.i64(ptr %slot, i8 0, i64 16, i1 false)
  store ptr %old, ptr %slot
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  %destination = getelementptr [2 x ptr], ptr %slot, i64 0, i64 %index
  store ptr null, ptr %destination
  %result = load ptr, ptr %slot
  ret ptr %result
}

; The shared transfer must not turn a same-address ABI-home copy into a kill.
define goabiinternal ptr @byval_self_copy(ptr byval(ptr) align 8 %slot) gc "goallc" {
entry:
  call goabiinternal void @checkpoint()
  call void @llvm.memmove.p0.p0.i64(ptr %slot, ptr %slot, i64 8, i1 false)
  %result = load ptr, ptr %slot
  ret ptr %result
}

; A loop-free PHI/select is itself the precise dynamic root for the selected
; stack object. Keep the candidate objects inactive so the unselected contents
; do not retain heap objects.
define goabiinternal ptr @mixed_alloca_phi(ptr %a, ptr %b, i1 %cond) gc "goallc" {
entry:
  %left = alloca ptr, align 8
  %right = alloca ptr, align 8
  store ptr %a, ptr %left
  store ptr %b, ptr %right
  br i1 %cond, label %take_left, label %take_right
take_left:
  br label %merge
take_right:
  br label %merge
merge:
  %selected = phi ptr [ %left, %take_left ], [ %right, %take_right ]
  call goabiinternal void @checkpoint()
  %result = load ptr, ptr %selected
  ret ptr %result
}

define goabiinternal ptr @mixed_alloca_select(ptr %a, ptr %b, i1 %cond) gc "goallc" {
entry:
  %left = alloca ptr, align 8
  %right = alloca ptr, align 8
  store ptr %a, ptr %left
  store ptr %b, ptr %right
  %selected = select i1 %cond, ptr %left, ptr %right
  call goabiinternal void @checkpoint()
  %result = load ptr, ptr %selected
  ret ptr %result
}

define goabiinternal ptr @mixed_byval_select(ptr byval(ptr) align 8 %arg,
                                             ptr %value, i1 %cond) gc "goallc" {
entry:
  %local = alloca ptr, align 8
  store ptr %value, ptr %local
  %selected = select i1 %cond, ptr %arg, ptr %local
  call goabiinternal void @checkpoint()
  %result = load ptr, ptr %selected
  ret ptr %result
}

; A loop-carried merge can select another object on a later iteration. That
; object's contents are not reachable through the current merged pointer, so
; attribute the downstream read to every candidate frame base.
define goabiinternal ptr @mixed_alloca_loop_phi(ptr %a, ptr %b) gc "goallc" {
entry:
  %left = alloca ptr, align 8
  %right = alloca ptr, align 8
  store ptr %a, ptr %left
  store ptr %b, ptr %right
  br label %loop
loop:
  %first = phi i1 [ true, %entry ], [ false, %next ]
  %selected = phi ptr [ %left, %entry ], [ %right, %next ]
  call goabiinternal void @checkpoint()
  %result = load ptr, ptr %selected
  br i1 %first, label %next, label %exit
next:
  call goabiinternal void @checkpoint()
  br label %loop
exit:
  ret ptr %result
}

; Deliberately place the exit before the loop in block order. Killing the first
; pointer slot must not hide the second slot's read across the backedge.
define goabiinternal ptr @loop_partial_overwrite(ptr %a, ptr %b, i1 %again) gc "goallc" {
entry:
  %slot = alloca %pair, align 8
  %second = getelementptr %pair, ptr %slot, i64 0, i32 1
  store ptr %a, ptr %slot
  store ptr %b, ptr %second
  call goabiinternal void @observe(ptr %slot)
  br label %loop
exit:
  %result = load ptr, ptr %second
  ret ptr %result
loop:
  call goabiinternal void @checkpoint()
  store ptr null, ptr %slot
  br i1 %again, label %loop, label %exit
}

; Neither loop block has a content use/def or lifetime event for slot.
; The exit read must propagate through both identity transfers and the
; backedge, even with the exit placed before the loop in block order.
define goabiinternal ptr @event_free_loop_live(ptr %old, i1 %again) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %old, ptr %slot
  call goabiinternal void @observe(ptr %slot)
  br label %loop
exit:
  %result = load ptr, ptr %slot
  ret ptr %result
loop:
  call goabiinternal void @checkpoint()
  br i1 %again, label %backedge, label %exit
backedge:
  call goabiinternal void @checkpoint()
  br label %loop
}

; An overwrite before the exit read kills the old contents. Identity
; transfers must also preserve an empty live set across the same loop.
define goabiinternal ptr @event_free_loop_killed(ptr %old, i1 %again) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %old, ptr %slot
  call goabiinternal void @observe(ptr %slot)
  br label %loop
exit:
  store ptr null, ptr %slot
  %result = load ptr, ptr %slot
  ret ptr %result
loop:
  call goabiinternal void @checkpoint()
  br i1 %again, label %backedge, label %exit
backedge:
  call goabiinternal void @checkpoint()
  br label %loop
}

; Building an aggregate is not the last use of its frame pointer. The read
; through the merged aggregate must keep contents written after the merge
; alive across the checkpoint, even though the frame address is rematerialized.
define goabiinternal ptr @aggregate_alloca_phi(ptr %old, i1 %cond) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  br i1 %cond, label %left, label %right
left:
  %a = insertvalue { ptr, i64 } { ptr poison, i64 1 }, ptr %slot, 0
  br label %join
right:
  %b = insertvalue { ptr, i64 } { ptr poison, i64 2 }, ptr %slot, 0
  br label %join
join:
  %merged = phi { ptr, i64 } [ %a, %left ], [ %b, %right ]
  store ptr %old, ptr %slot
  call goabiinternal void @checkpoint()
  %address = extractvalue { ptr, i64 } %merged, 0
  %result = load ptr, ptr %address
  ret ptr %result
}

; Fixed ABI homes use the same content analysis, including projections from
; nested aggregates forwarded through select/freeze.
define goabiinternal ptr @aggregate_byval_select(ptr byval(ptr) align 8 %slot,
                                                ptr %old, i1 %cond) gc "goallc" {
entry:
  %a = insertvalue { i64, { ptr, i64 } } { i64 1, { ptr, i64 } poison }, ptr %slot, 1, 0
  %b = insertvalue { i64, { ptr, i64 } } { i64 2, { ptr, i64 } poison }, ptr %slot, 1, 0
  %selected = select i1 %cond, { i64, { ptr, i64 } } %a, { i64, { ptr, i64 } } %b
  %merged = freeze { i64, { ptr, i64 } } %selected
  store ptr %old, ptr %slot
  call goabiinternal void @checkpoint()
  %address = extractvalue { i64, { ptr, i64 } } %merged, 1, 0
  %result = load ptr, ptr %address
  ret ptr %result
}

; Volatile loads keep these memory round trips explicit. The first case
; keeps the carrier live; the other two clear it before the checkpoint.
define goabiinternal ptr @memory_live(ptr %p) gc "goallc" {
entry:
  %box = alloca { ptr, i64 }, align 8
  %value = insertvalue { ptr, i64 } { ptr poison, i64 1 }, ptr %p, 0
  store { ptr, i64 } %value, ptr %box
  call goabiinternal void @checkpoint()
  %loaded = load volatile { ptr, i64 }, ptr %box
  %result = extractvalue { ptr, i64 } %loaded, 0
  ret ptr %result
}

define goabiinternal ptr @loaded_heap_pointer(ptr %p) gc "goallc" {
entry:
  %box = alloca { ptr, i64 }, align 8
  %value = insertvalue { ptr, i64 } { ptr poison, i64 1 }, ptr %p, 0
  store { ptr, i64 } %value, ptr %box
  %loaded = load volatile { ptr, i64 }, ptr %box
  store { ptr, i64 } zeroinitializer, ptr %box
  call goabiinternal void @checkpoint()
  %result = extractvalue { ptr, i64 } %loaded, 0
  ret ptr %result
}

define goabiinternal ptr @loaded_stack_pointer(ptr %p) gc "goallc" {
entry:
  %target = alloca ptr, align 8
  %box = alloca { ptr, i64 }, align 8
  %value = insertvalue { ptr, i64 } { ptr poison, i64 1 }, ptr %target, 0
  store { ptr, i64 } %value, ptr %box
  %loaded = load volatile { ptr, i64 }, ptr %box
  store { ptr, i64 } zeroinitializer, ptr %box
  store ptr %p, ptr %target
  call goabiinternal void @checkpoint()
  %address = extractvalue { ptr, i64 } %loaded, 0
  %result = load ptr, ptr %address
  ret ptr %result
}
