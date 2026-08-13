target triple = "x86_64-unknown-linux-goobj"

; IR-LABEL: define goabiinternal ptr @pointer_slot(
; IR: "deopt"(i64 7, i64 1195461697, i64 15, i64 1, i64 1095520067, i64 11, ptr %slot, i64 0, i64 8, i64 8, i64 8, i64 1, i64 64, i64 1, i64 1, i64 1095519299, i64 15)
; IR-SAME: "gc-live"(ptr %slot)

; IR-LABEL: define goabiinternal ptr @nested_whole_aggregate(
; IR: "deopt"(i64 1195461697, i64 15, i64 1, i64 1095520067, i64 11, ptr %slot, i64 0, i64 48, i64 8, i64 8, i64 6, i64 64, i64 1, i64 41, i64 1095519299, i64 15)

; IR-LABEL: define goabiinternal ptr @alloca_call_skip(
; IR: "deopt"({{.*}}i64 1095520067{{.*}}ptr %slot{{.*}}i64 1095519299

; IR-LABEL: define goabiinternal ptr @alloca_multiple_calls(
; IR-COUNT-2: "deopt"({{.*}}i64 1095520067{{.*}}ptr %slot{{.*}}i64 1095519299
; IR-NOT: store ptr null

; IR-LABEL: define goabiinternal ptr @alloca_partial_initialization()
; IR: call void @llvm.lifetime.start
; IR-NEXT: call void @llvm.memset.inline
; IR: @llvm.experimental.gc.statepoint{{.*}}"deopt"({{.*}}ptr %slot
; IR: @llvm.experimental.gc.statepoint{{.*}}"deopt"({{.*}}ptr %slot

; IR-LABEL: define goabiinternal ptr @alloca_loop(
; IR: "deopt"({{.*}}i64 1095520067{{.*}}ptr %slot{{.*}}i64 1095519299

; IR-LABEL: define goabiinternal ptr @alloca_gep_address_across_call(
; IR: "deopt"({{.*}}ptr %slot{{.*}}i64 16{{.*}}i64 2{{.*}}i64 2{{.*}}i64 1095519299
; IR: %result = load ptr, ptr %field
; IR-NOT: %field.remat
; IR-NOT: %field.relocated.merge

; IR-LABEL: define goabiinternal void @alloca_direct_address_across_calls()
; IR: "gc-live"(ptr %slot)
; IR-COUNT-2: getelementptr inbounds i8, ptr %slot, i64 0
; IR-NOT: .address.relocated.merge

; IR-LABEL: define goabiinternal void @alloca_gep_value_across_calls()
; IR: %field.remat{{[0-9]+}} = getelementptr inbounds %pointer_field, ptr %slot
; IR: "gc-live"(ptr %slot)
; IR: %field.remat = getelementptr inbounds %pointer_field, ptr %slot
; IR-NOT: .remat.relocated.merge

; IR-LABEL: define goabiinternal void @alloca_pointer_free_address_across_calls()
; IR-NOT: "deopt"(
; IR-NOT: "gc-live"(
; IR-COUNT-2: getelementptr inbounds i8, ptr %slot, i64 0

; IR-LABEL: define goabiinternal ptr @alloca_address_passed_to_callee(
; IR: store ptr %pointer, ptr %slot
; IR-NOT: store ptr null, ptr %slot
; IR: @llvm.experimental.gc.statepoint{{.*}}"deopt"({{.*}}ptr %slot{{.*}}){{.*}}"gc-live"(ptr %slot)
; IR: %slot.relocated = call coldcc ptr @llvm.experimental.gc.relocate
; IR-NOT: store ptr {{.*}}, ptr %slot

; IR-LABEL: define goabiinternal void @alloca_marker_free_at_safepoint(
; IR: i64 1095519299, i64 15), "gc-live"(ptr %pointer{{[,)]}}
; IR: %pointer.relocated

; IR-LABEL: define goabiinternal ptr @alloca_high_bitmap_word(
; IR: "deopt"({{.*}}ptr %slot{{.*}}i64 512{{.*}}i64 64{{.*}}i64 64{{.*}}i64 1{{.*}}i64 -9223372036854775808

; IR-LABEL: define goabiinternal ptr @alloca_multiple_records(
; IR: "deopt"({{.*}}i64 1, i64 1095520067{{.*}}ptr %left

; IR-LABEL: define goabiinternal ptr @alloca_select_same_base(
; IR: "deopt"({{.*}}ptr %slot
; IR-SAME: "gc-live"(ptr %selected
; IR: %selected.relocated

; MIR-COUNT-31: STATEPOINT{{.*}}1195461697{{.*}}1095520067{{.*}}%{{(fixed-)?}}stack.{{[0-9]+}}
; MIR-ALL-COUNT-34: STATEPOINT

; O2-LABEL: define goabiinternal ptr @nested_whole_aggregate(
; O2-NOT: alloca %nested
; O2-LABEL: define goabiinternal ptr @alloca_call_skip(
; O2: "deopt"({{.*}}i64 1095520067

; OBJVIEW-LABEL: "name": "alloca_multiple_calls"
; OBJVIEW-NOT: "kind": "stack_objects"
; OBJVIEW: "kind": "args_pointer_maps"
; OBJVIEW: "set_bits": [
; OBJVIEW: "kind": "locals_pointer_maps"
; OBJVIEW: "set_bits": null
; OBJVIEW-LABEL: "name": "alloca_loop"

; OBJVIEW-LABEL: "name": "alloca_direct_address_across_calls"
; OBJVIEW: "kind": "locals_pointer_maps"
; OBJVIEW: "set_bits": [
; OBJVIEW: "kind": "stack_objects"
; OBJVIEW-LABEL: "name": "argument_home_address_across_calls"
; OBJVIEW: "kind": "args_pointer_maps"
; OBJVIEW: "set_bits": [
; OBJVIEW: "kind": "stack_objects"
; OBJVIEW: "offset": 0
; OBJVIEW: "size": 8
; OBJVIEW: "ptr_bytes": 8
; OBJVIEW-LABEL: "name": "argument_aggregate_home_address_across_calls"
; OBJVIEW: "kind": "args_pointer_maps"
; OBJVIEW: "set_bits": [
; OBJVIEW-NEXT: 0,
; OBJVIEW-NEXT: 3,
; OBJVIEW-NEXT: 5
; OBJVIEW: "kind": "stack_objects"
; OBJVIEW: "offset": 0
; OBJVIEW: "size": 48
; OBJVIEW: "ptr_bytes": 48
; OBJVIEW: "name": "runtime.gcbits.2900000000000000"

; OBJVIEW-LABEL: "name": "alloca_pointer_free_address_across_calls"
; OBJVIEW-NOT: "kind": "stack_objects"
; OBJVIEW: "kind": "locals_pointer_maps"
; OBJVIEW: "set_bits": null
; OBJVIEW-LABEL: "name": "alloca_address_passed_to_callee"
; OBJVIEW-NOT: "kind": "stack_objects"
; OBJVIEW: "kind": "args_pointer_maps"
; OBJVIEW: "set_bits": [
; OBJVIEW-LABEL: "name": "alloca_marker_free_at_safepoint"

%nested = type { ptr, i64, [2 x { i32, ptr }] }
%pointer_field = type { i64, ptr }
%two_pointers = type { ptr, ptr }
%high_bitmap = type { [63 x i64], ptr }

declare goabiinternal void @safepoint()
declare goabiinternal void @mutate_pointer_slot(ptr)
declare goabiinternal void @mutate_nocapture(ptr captures(none))
declare goabiinternal void @escape_pointer_slot(ptr)
declare goabiinternal void @unknown_writing()
declare goabiinternal ptr @make_pointer()
declare goabiinternal void @observe_stack_address(ptr)
declare goabiinternal i64 @readonly_pointer_slot(ptr readonly) memory(read)
declare goabiinternal i64 @readnone_callee() memory(none)
declare void @llvm.lifetime.start.p0(i64 immarg, ptr captures(none))

define goabiinternal ptr @pointer_slot(ptr %pointer) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %pointer, ptr %slot, align 8
  %nilcheck = load volatile i8, ptr %slot, align 1, !goallc.nilcheck !0
  call goabiinternal void @safepoint() [ "deopt"(i64 7) ]
  %result = load ptr, ptr %slot, align 8
  ret ptr %result
}

define goabiinternal ptr @nested_whole_aggregate(
    ptr %first, ptr %second, ptr %third) gc "goallc" {
entry:
  ; The optimized use graph contains only direct memory operations, so this
  ; must use fixed homes and remain eligible for SROA.
  %slot = alloca %nested, align 8
  %value.0 = insertvalue %nested zeroinitializer, ptr %first, 0
  %value.1 = insertvalue %nested %value.0, ptr %second, 2, 0, 1
  %value.2 = insertvalue %nested %value.1, ptr %third, 2, 1, 1
  store %nested %value.2, ptr %slot, align 8
  call goabiinternal void @safepoint()
  %reloaded = load %nested, ptr %slot, align 8
  %result = extractvalue %nested %reloaded, 2, 1, 1
  ret ptr %result
}

define goabiinternal ptr @alloca_call_skip(
    i1 %take_call, ptr %pointer) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %pointer, ptr %slot, align 8
  br i1 %take_call, label %call, label %merge

call:
  call goabiinternal void @safepoint()
  br label %merge

merge:
  %result = load ptr, ptr %slot, align 8
  ret ptr %result
}

define goabiinternal ptr @alloca_multiple_calls(
    ptr %pointer) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %pointer, ptr %slot, align 8
  call goabiinternal void @safepoint()
  call goabiinternal void @safepoint()
  %result = load ptr, ptr %slot, align 8
  ret ptr %result
}

define goabiinternal ptr @alloca_partial_initialization()
    gc "goallc" {
entry:
  ; Each field is initialized from a different safepointing call. The plugin
  ; must zero the whole object before the first call so the complete bitmap is
  ; safe both before initialization and while only the first field is set.
  %slot = alloca %two_pointers, align 8
  call void @llvm.lifetime.start.p0(i64 16, ptr %slot)
  %first = call goabiinternal ptr @make_pointer()
  %first.field = getelementptr inbounds %two_pointers, ptr %slot, i32 0, i32 0
  store ptr %first, ptr %first.field, align 8
  %second = call goabiinternal ptr @make_pointer()
  %second.field = getelementptr inbounds %two_pointers, ptr %slot, i32 0, i32 1
  store ptr %second, ptr %second.field, align 8
  %result = load ptr, ptr %first.field, align 8
  ret ptr %result
}

define goabiinternal ptr @alloca_loop(
    i1 %again, ptr %pointer) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %pointer, ptr %slot, align 8
  br label %loop

loop:
  call goabiinternal void @safepoint()
  br i1 %again, label %loop, label %exit

exit:
  %result = load ptr, ptr %slot, align 8
  ret ptr %result
}

define goabiinternal ptr @alloca_gep_address_across_call(
    ptr %pointer) gc "goallc" {
entry:
  %slot = alloca %pointer_field, align 8
  %field = getelementptr inbounds %pointer_field, ptr %slot, i32 0, i32 1
  store ptr %pointer, ptr %field, align 8
  call goabiinternal void @safepoint()
  %result = load ptr, ptr %field, align 8
  ret ptr %result
}

define goabiinternal void @alloca_direct_address_across_calls()
    gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr null, ptr %slot, align 8
  call goabiinternal void @observe_stack_address(ptr %slot)
  call goabiinternal void @safepoint()
  call goabiinternal void @observe_stack_address(ptr %slot)
  ret void
}

define goabiinternal void @argument_home_address_across_calls(ptr %pointer)
    gc "goallc" {
entry:
  ; This canonical parameter alloca becomes the argument's fixed ABI home.
  ; Its last callsite has no direct gc-live base, so the function also needs
  ; an argp-relative StackObject for pointers that reach the home indirectly.
  %slot = alloca ptr, align 8
  call void @llvm.lifetime.start.p0(i64 8, ptr %slot)
  store ptr %pointer, ptr %slot, align 8
  call goabiinternal void @observe_stack_address(ptr %slot)
  call goabiinternal void @safepoint()
  call goabiinternal void @observe_stack_address(ptr %slot)
  ret void
}

define goabiinternal void @argument_aggregate_home_address_across_calls(
    %nested %value) gc "goallc" {
entry:
  ; The split aggregate parameter still has one complete fixed home and one
  ; argp-relative StackObject covering all ABI pieces and padding.
  %slot = alloca %nested, align 8
  call void @llvm.lifetime.start.p0(i64 48, ptr %slot)
  store %nested %value, ptr %slot, align 8
  call goabiinternal void @observe_stack_address(ptr %slot)
  call goabiinternal void @safepoint()
  call goabiinternal void @observe_stack_address(ptr %slot)
  ret void
}

define goabiinternal void @alloca_gep_value_across_calls()
    gc "goallc" {
entry:
  %slot = alloca %pointer_field, align 8
  %field = getelementptr inbounds %pointer_field, ptr %slot, i32 0, i32 1
  store ptr null, ptr %field, align 8
  call goabiinternal void @observe_stack_address(ptr %field)
  call goabiinternal void @safepoint()
  call goabiinternal void @observe_stack_address(ptr %field)
  ret void
}

define goabiinternal void @alloca_pointer_free_address_across_calls()
    gc "goallc" {
entry:
  %slot = alloca i64, align 8
  store i64 0, ptr %slot, align 8
  call goabiinternal void @observe_stack_address(ptr %slot)
  call goabiinternal void @safepoint()
  call goabiinternal void @observe_stack_address(ptr %slot)
  ret void
}

define goabiinternal ptr @alloca_address_passed_to_callee(
    ptr %pointer) gc "goallc" {
entry:
  ; The structural call use makes the address observable.
  %slot = alloca ptr, align 8
  store ptr %pointer, ptr %slot, align 8
  call goabiinternal void @mutate_pointer_slot(ptr %slot)
  %result = load ptr, ptr %slot, align 8
  ret ptr %result
}

define goabiinternal void @alloca_marker_free_at_safepoint(
    ptr %pointer) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr null, ptr %slot, align 8
  call goabiinternal void @safepoint()
  store ptr %pointer, ptr %slot, align 8
  ret void
}

define goabiinternal ptr @alloca_high_bitmap_word(
    ptr %pointer) gc "goallc" {
entry:
  %slot = alloca %high_bitmap, align 8
  %field = getelementptr inbounds %high_bitmap, ptr %slot, i32 0, i32 1
  store ptr %pointer, ptr %field, align 8
  call goabiinternal void @safepoint()
  %result = load ptr, ptr %field, align 8
  ret ptr %result
}

define goabiinternal ptr @alloca_multiple_records(
    ptr %first, ptr %second) gc "goallc" {
entry:
  %left = alloca ptr, align 8
  %right = alloca ptr, align 8
  store ptr %first, ptr %left, align 8
  store ptr %second, ptr %right, align 8
  call goabiinternal void @safepoint()
  %result = load ptr, ptr %left, align 8
  ret ptr %result
}

define goabiinternal ptr @alloca_select_same_base(
    i1 %choose, ptr %pointer) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  %same = getelementptr inbounds i8, ptr %slot, i64 0
  %selected = select i1 %choose, ptr %slot, ptr %same
  store ptr %pointer, ptr %selected, align 8
  call goabiinternal void @safepoint()
  %result = load ptr, ptr %selected, align 8
  ret ptr %result
}

define goabiinternal ptr @alloca_nocapture_writable(
    ptr %pointer) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %pointer, ptr %slot, align 8
  call goabiinternal void @mutate_nocapture(ptr captures(none) %slot)
  %result = load ptr, ptr %slot, align 8
  ret ptr %result
}

define goabiinternal ptr @alloca_escaped_before_unknown_write(
    ptr %pointer) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %pointer, ptr %slot, align 8
  call goabiinternal void @escape_pointer_slot(ptr %slot)
  call goabiinternal void @unknown_writing()
  %result = load ptr, ptr %slot, align 8
  ret ptr %result
}

define goabiinternal i64 @alloca_readonly_and_readnone(
    ptr %pointer) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %pointer, ptr %slot, align 8
  %read = call goabiinternal i64 @readonly_pointer_slot(ptr readonly %slot)
  %pure = call goabiinternal i64 @readnone_callee()
  %result = load ptr, ptr %slot, align 8
  %bits = ptrtoint ptr %result to i64
  %sum.0 = add i64 %read, %pure
  %sum.1 = add i64 %sum.0, %bits
  ret i64 %sum.1
}

@llvm.used = appending global [19 x ptr] [
  ptr @pointer_slot,
  ptr @nested_whole_aggregate,
  ptr @alloca_call_skip,
  ptr @alloca_multiple_calls,
  ptr @alloca_loop,
  ptr @alloca_gep_address_across_call,
  ptr @alloca_direct_address_across_calls,
  ptr @argument_home_address_across_calls,
  ptr @argument_aggregate_home_address_across_calls,
  ptr @alloca_gep_value_across_calls,
  ptr @alloca_pointer_free_address_across_calls,
  ptr @alloca_address_passed_to_callee,
  ptr @alloca_marker_free_at_safepoint,
  ptr @alloca_high_bitmap_word,
  ptr @alloca_multiple_records,
  ptr @alloca_select_same_base,
  ptr @alloca_nocapture_writable,
  ptr @alloca_escaped_before_unknown_write,
  ptr @alloca_readonly_and_readnone
], section "llvm.metadata"

!0 = !{}
