target triple = "aarch64-apple-darwin-goobj"

%result_storage = type { ptr, i64, ptr }
%quad_pointers = type { ptr, ptr, ptr, ptr }

; IR-LABEL: define goabiinternal {{.*}} @fixed_frame_goret_base(
; IR-NOT: @llvm.experimental.gc.relocate
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %result)
; IR: store %result_storage zeroinitializer, ptr %result, align 8
; IR: "deopt"({{.*}}ptr %result{{.*}}i64 1{{.*}}i64 3{{.*}}i64 5{{.*}}"gc-live"(ptr %result)
; IR-NOT: @llvm.experimental.gc.relocate

; Goret contents have a forward lifetime: they are inactive before definite
; initialization, then remain active through later overwrites until return.
; IR-LABEL: define goabiinternal {{.*}} @fixed_frame_goret_initialization_interval(
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"({{.*}}ptr %result)
; IR: store %result_storage zeroinitializer, ptr %result, align 8
; IR: "deopt"({{.*}}ptr %result{{.*}}i64 1{{.*}}i64 3{{.*}}i64 5{{.*}}"gc-live"({{.*}}ptr %result)
; IR: store ptr %replacement{{.*}}, ptr %result, align 8
; IR: "deopt"({{.*}}ptr %result{{.*}}i64 1{{.*}}i64 3{{.*}}i64 5{{.*}}"gc-live"(ptr %result)
; IR-NOT: %result.relocated = call coldcc ptr @llvm.experimental.gc.relocate

; A named result observed by defer is exceptional: recovery can resume outside
; LLVM's explicit CFG, so its contents remain active for the whole function,
; including safepoints before the ordinary goret initialization point.
; IR-LABEL: define goabiinternal {{.*}} @fixed_frame_defer_goret_whole_function(
; IR-COUNT-2: "deopt"({{.*}}ptr %result{{.*}}i64 1{{.*}}i64 3{{.*}}i64 5{{.*}}"gc-live"(ptr %result)
; IR-NOT: %result.relocated = call coldcc ptr @llvm.experimental.gc.relocate

; Byval and goret derived addresses use the same Base+Offset representation as
; allocas. Only the integer choice crosses the statepoint; each continuation
; rebuilds the concrete address from the canonical fixed argument FrameIndex.
; IR-LABEL: define goabiinternal void @fixed_frame_byval_different_offset(
; IR: %address.offset = select i1 %choose, i64 0, i64 8
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %input)
; IR-NOT: %input.relocated = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %address.remat = getelementptr i8, ptr %input, i64 %address.offset
; IR-NEXT: %value = load ptr, ptr %address.remat

; IR-LABEL: define goabiinternal {{.*}} @fixed_frame_goret_different_offset(
; IR: %address.offset = select i1 %choose, i64 0, i64 16
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"({{.*}}ptr %result)
; IR-NOT: %result.relocated = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %address.remat = getelementptr i8, ptr %result, i64 %address.offset
; IR: store ptr %replacement{{.*}}, ptr %address.remat

; IR-LABEL: define goabiinternal void @fixed_frame_byval_contents(
; IR: "deopt"({{.*}}ptr %input{{.*}}i64 1{{.*}}i64 3{{.*}}i64 5{{.*}}"gc-live"(ptr %input)
; IR: load ptr, ptr %input, align 8
; IR-NOT: "deopt"({{.*}}ptr %input

; A store selected between distinct pointer slots cannot kill either slot on
; every path. Both surrounding statepoints therefore retain the byval
; contents. The optimized encoding/json/v2 reproducer has this shape.
; IR-LABEL: define goabiinternal void @fixed_frame_byval_phi_offsets(
; IR-COUNT-2: "deopt"({{.*}}ptr %input{{.*}}i64 1{{.*}}i64 4{{.*}}i64 15{{.*}}"gc-live"({{.*}}ptr %input)
; IR-NOT: ; (%input, %input)

; An offset which cannot be enumerated is also conservative: the store has no
; must-def pointer slots, rather than rejecting the function.
; IR-LABEL: define goabiinternal void @fixed_frame_byval_unknown_offset(
; IR-COUNT-2: "deopt"({{.*}}ptr %input{{.*}}i64 1{{.*}}i64 4{{.*}}i64 15{{.*}}"gc-live"({{.*}}ptr %input)
; IR-NOT: ; (%input, %input)

; A loop-carried aggregate still receives normal relocation SSA at the call,
; but its pointer leaf would otherwise become a Machine PHI. Reinsert the
; fixed argument immediately at the statepoint so ordinary fixed-argument
; lowering selects the FrameIndex.
; IR-LABEL: define goabiinternal void @fixed_frame_byval_aggregate_loop(
; IR: %argument.relocated.merge.0 = phi { ptr }
; IR: %[[BYVAL_FIXED:[-a-zA-Z$._0-9]+]] = insertvalue { ptr } %argument.relocated.merge.0, ptr %input, 0
; IR-NEXT: %statepoint_token = call goabiinternal token {{.*}}@llvm.experimental.gc.statepoint{{.*}}@consume_pointer_aggregate{{.*}}%[[BYVAL_FIXED]]
; IR-NOT: llvm.stackaddress
; IR-NOT: ptrtoint ptr %input

; IR-LABEL: define goabiinternal {{.*}} @fixed_frame_goret_aggregate_loop(
; IR: %argument.relocated.merge.0 = phi { ptr }
; IR: %[[GORET_FIXED:[-a-zA-Z$._0-9]+]] = insertvalue { ptr } %argument.relocated.merge.0, ptr %result, 0
; IR-NEXT: %statepoint_token = call goabiinternal token {{.*}}@llvm.experimental.gc.statepoint{{.*}}@consume_pointer_aggregate{{.*}}%[[GORET_FIXED]]
; IR-NOT: llvm.stackaddress
; IR-NOT: ptrtoint ptr %result

; MIR-LABEL: name: fixed_frame_goret_base
; MIR: fixedStack:
; MIR: type: default
; MIR: stack: []
; MIR: bb.0.entry:
; MIR: STATEPOINT{{.*}}2, 1, 0, %fixed-stack.0, 0, 2, 0, csr_aarch64_go,
; MIR: bb.1.entry.statepoint.cont:
; MIR: [[RESULT:%[0-9]+]]:gpr64 = COPY $xzr
; MIR: STRXui [[RESULT]], %fixed-stack.0, 0
; MIR: STATEPOINT{{.*}}2, 1, 0, %fixed-stack.0, 0, 2, 0, csr_aarch64_go,
; MIR: bb.2.entry.statepoint.cont.statepoint.cont:

; MIR-LABEL: name: fixed_frame_byval_aggregate_loop
; MIR: bb.{{[0-9]+}}.loop:
; MIR: %[[BYVAL_FRAME:[0-9]+]]:{{[a-z0-9_]+}} = ADDXri %fixed-stack.{{[0-9]+}}
; MIR: $x0 = COPY %[[BYVAL_FRAME]]
; MIR: STATEPOINT{{.*}}@consume_pointer_aggregate

; MIR-LABEL: name: fixed_frame_goret_aggregate_loop
; MIR: bb.{{[0-9]+}}.loop:
; MIR: %[[GORET_FRAME:[0-9]+]]:{{[a-z0-9_]+}} = ADDXri %fixed-stack.{{[0-9]+}}
; MIR: $x0 = COPY %[[GORET_FRAME]]
; MIR: STATEPOINT{{.*}}@consume_pointer_aggregate

; OBJVIEW-LABEL: "name": "fixed_frame_goret_base"
; OBJVIEW: "kind": "args_pointer_maps"
; OBJVIEW: "count": 2
; OBJVIEW: "num_bits": 3
; OBJVIEW: "set_bits": null
; OBJVIEW: "set_bits": [
; OBJVIEW-NEXT: 0,
; OBJVIEW-NEXT: 2
; OBJVIEW: "kind": "locals_pointer_maps"
; OBJVIEW: "count": 2
; OBJVIEW: "set_bits": null

; The first bitmap is the entry/prologue map for the ordinary replacement
; argument. The three explicit safepoints then see goret as inactive, active,
; and still active after an overwrite.
; OBJVIEW-LABEL: "name": "fixed_frame_goret_initialization_interval"
; OBJVIEW: "kind": "args_pointer_maps"
; OBJVIEW: "count": 4
; OBJVIEW: "num_bits": 4
; OBJVIEW: "set_bits": [
; OBJVIEW-NEXT: 3
; OBJVIEW: "set_bits": null
; OBJVIEW: "set_bits": [
; OBJVIEW-NEXT: 0,
; OBJVIEW-NEXT: 2
; OBJVIEW: "set_bits": [
; OBJVIEW-NEXT: 0,
; OBJVIEW-NEXT: 2

; Both explicit safepoints use bitmap 1 below; bitmap 0 is the entry/prologue
; map. This confirms that defer keeps the result contents active even at the
; call before the ordinary initialization store.
; OBJVIEW-LABEL: "name": "fixed_frame_defer_goret_whole_function"
; OBJVIEW: "kind": "args_pointer_maps"
; OBJVIEW: "count": 2
; OBJVIEW: "num_bits": 3
; OBJVIEW: "set_bits": null
; OBJVIEW: "set_bits": [
; OBJVIEW-NEXT: 0,
; OBJVIEW-NEXT: 2
; OBJVIEW-COUNT-2: "stack_map_index": 1

; OBJVIEW-LABEL: "name": "fixed_frame_byval_contents"
; OBJVIEW: "kind": "args_pointer_maps"
; OBJVIEW: "count": 2
; OBJVIEW: "num_bits": 3
; OBJVIEW: "set_bits": [
; OBJVIEW-NEXT: 0,
; OBJVIEW-NEXT: 2
; OBJVIEW: "set_bits": null

declare goabiinternal void @safepoint()
declare goabiinternal void @consume_pointer_aggregate({ ptr })
declare void @observe_pointer(ptr) #1

define goabiinternal { i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 } @fixed_frame_goret_base(
    ptr goret(%result_storage) align 8 "goretindex"="15" %result) #0 gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  store %result_storage zeroinitializer, ptr %result, align 8
  call goabiinternal void @safepoint()
  ret { i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 } zeroinitializer
}

define goabiinternal { i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 } @fixed_frame_goret_initialization_interval(
    ptr %replacement,
    ptr goret(%result_storage) align 8 "goretindex"="15" %result) #0 gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  store %result_storage zeroinitializer, ptr %result, align 8
  call goabiinternal void @safepoint()
  store ptr %replacement, ptr %result, align 8
  call goabiinternal void @safepoint()
  ret { i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 } zeroinitializer
}

define goabiinternal { i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 } @fixed_frame_defer_goret_whole_function(
    ptr goret(%result_storage) align 8 "goretindex"="15" "goallc.defer_result" %result) #0 gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  store %result_storage zeroinitializer, ptr %result, align 8
  call goabiinternal void @safepoint()
  ret { i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 } zeroinitializer
}

define goabiinternal void @fixed_frame_byval_different_offset(
    ptr byval([2 x ptr]) align 8 %input, i1 %choose) #0 gc "goallc" {
entry:
  %first = getelementptr inbounds [2 x ptr], ptr %input, i64 0, i64 0
  %second = getelementptr inbounds [2 x ptr], ptr %input, i64 0, i64 1
  %address = select i1 %choose, ptr %first, ptr %second
  call goabiinternal void @safepoint()
  %value = load ptr, ptr %address, align 8
  call void @observe_pointer(ptr %value)
  ret void
}

define goabiinternal { i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 } @fixed_frame_goret_different_offset(
    ptr %replacement, i1 %choose,
    ptr goret(%result_storage) align 8 "goretindex"="15" %result) #0 gc "goallc" {
entry:
  store %result_storage zeroinitializer, ptr %result, align 8
  %first = getelementptr inbounds %result_storage, ptr %result, i64 0, i32 0
  %second = getelementptr inbounds %result_storage, ptr %result, i64 0, i32 2
  %address = select i1 %choose, ptr %first, ptr %second
  call goabiinternal void @safepoint()
  store ptr %replacement, ptr %address, align 8
  ret { i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 } zeroinitializer
}

define goabiinternal void @fixed_frame_byval_contents(
    ptr byval(%result_storage) align 8 %input) #0 gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  %value = load ptr, ptr %input, align 8
  call goabiinternal void @safepoint()
  ret void
}

define goabiinternal void @fixed_frame_byval_phi_offsets(
    ptr byval(%quad_pointers) align 8 %input, i1 %condition,
    ptr %replacement) #0 gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  br i1 %condition, label %left, label %right

left:
  br label %merge

right:
  br label %merge

merge:
  %offset = phi i64 [ 0, %left ], [ 16, %right ]
  %address = getelementptr i8, ptr %input, i64 %offset
  store ptr %replacement, ptr %address, align 8
  call goabiinternal void @safepoint()
  %value = load ptr, ptr %input, align 8
  call void @observe_pointer(ptr %value)
  ret void
}

define goabiinternal void @fixed_frame_byval_unknown_offset(
    ptr byval(%quad_pointers) align 8 %input, i64 %offset,
    ptr %replacement) #0 gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  %address = getelementptr i8, ptr %input, i64 %offset
  store ptr %replacement, ptr %address, align 8
  call goabiinternal void @safepoint()
  %value = load ptr, ptr %input, align 8
  call void @observe_pointer(ptr %value)
  ret void
}

define goabiinternal void @fixed_frame_byval_aggregate_loop(
    ptr byval(ptr) align 8 %input, i1 %again) #0 gc "goallc" {
entry:
  %argument = insertvalue { ptr } poison, ptr %input, 0
  br label %loop

loop:
  call goabiinternal void @consume_pointer_aggregate({ ptr } %argument)
  br i1 %again, label %loop, label %exit

exit:
  ret void
}

define goabiinternal { i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 } @fixed_frame_goret_aggregate_loop(
    i1 %again,
    ptr goret(%result_storage) align 8 "goretindex"="15" %result) #0 gc "goallc" {
entry:
  store %result_storage zeroinitializer, ptr %result, align 8
  %argument = insertvalue { ptr } poison, ptr %result, 0
  br label %loop

loop:
  call goabiinternal void @consume_pointer_aggregate({ ptr } %argument)
  br i1 %again, label %loop, label %exit

exit:
  ret { i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 } zeroinitializer
}

attributes #0 = { "frame-pointer"="non-leaf" "go_results_tuple" }
attributes #1 = { "gc-leaf-function" }
