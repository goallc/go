target triple = "aarch64-apple-darwin-goobj"

%result_storage = type { ptr, i64, ptr }

; IR-LABEL: define goabiinternal {{.*}} @fixed_frame_goret_base(
; IR-COUNT-2: "gc-live"(ptr %result)

; MIR-LABEL: name: fixed_frame_goret_base
; MIR: fixedStack:
; MIR: type: default
; MIR: stack: []
; MIR: bb.0.entry:
; MIR: STATEPOINT{{.*}}%fixed-stack.0{{.*}}%fixed-stack.0
; MIR: bb.1.entry.statepoint.cont:
; MIR: STATEPOINT{{.*}}%fixed-stack.0{{.*}}%fixed-stack.0
; MIR: bb.2.entry.statepoint.cont.statepoint.cont:
; MIR-NOT: ADDXri %fixed-stack.0
; MIR: STRXui {{.*}}, %fixed-stack.0, 0

; OBJVIEW-LABEL: "name": "fixed_frame_goret_base"
; OBJVIEW: "kind": "locals_pointer_maps"
; OBJVIEW: "count": 1
; OBJVIEW: "set_bits": null

declare goabiinternal void @safepoint()
declare goabiinternal void @consume_slice({ ptr, i64, i64 })

define goabiinternal { i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 } @fixed_frame_goret_base(
    ptr goret(%result_storage) align 8 "goretindex"="15" %result) #0 gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  call goabiinternal void @safepoint()
  store ptr null, ptr %result, align 8
  ret { i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 } zeroinitializer
}

; IR-LABEL: define goabiinternal {{.*}} @fixed_frame_goret_loop(
; IR: %result.relocated.merge{{.*}} = phi ptr [ %result, %entry ], [ %result.relocated{{[0-9]*}}, %loop.statepoint.cont ]
; IR: store ptr null, ptr %result.relocated.merge
; IR: "gc-live"(ptr %result)

; MIR-LABEL: name: fixed_frame_goret_loop
; MIR: bb.1.loop:
; MIR-NOT: PHI
; MIR: STRXui {{.*}}, %fixed-stack.0, 0
; MIR: STATEPOINT

define goabiinternal { i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 } @fixed_frame_goret_loop(
    i1 %again,
    ptr goret(%result_storage) align 8 "goretindex"="15" %result) #0 gc "goallc" {
entry:
  br label %loop

loop:
  store ptr null, ptr %result, align 8
  call goabiinternal void @safepoint()
  br i1 %again, label %loop, label %exit

exit:
  ret { i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 } zeroinitializer
}

; An aggregate leaf can carry the exact address of a fixed alloca around the
; loop. Its relocate is also a reaching definition of the alloca's current
; frame base, so the next iteration's direct store must use that definition.
;
; IR-LABEL: define goabiinternal void @fixed_frame_aggregate_leaf_loop(
; IR: %slice.leaf.0.relocated.merge{{.*}} = phi ptr
; IR: %slot.relocated.merge{{.*}} = phi ptr [ %slot, %entry ], [ %slice.leaf.0.relocated{{[0-9]*}}, %loop.statepoint.cont ]
; IR: store ptr null, ptr %slot.relocated.merge
; IR: "gc-live"(ptr %slice.leaf.0.relocated.merge

; MIR-LABEL: name: fixed_frame_aggregate_leaf_loop
; MIR: bb.1.loop:
; MIR-NOT: PHI
; MIR: STRXui {{.*}}, %stack.0.slot, 0
; MIR: STATEPOINT

define goabiinternal void @fixed_frame_aggregate_leaf_loop(i1 %again) #1 gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  %slot.address = getelementptr i8, ptr %slot, i64 0
  %slice.0 = insertvalue { ptr, i64, i64 } undef, ptr %slot.address, 0
  %slice.1 = insertvalue { ptr, i64, i64 } %slice.0, i64 1, 1
  %slice = insertvalue { ptr, i64, i64 } %slice.1, i64 1, 2
  br label %loop

loop:
  call void @llvm.lifetime.start.p0(i64 8, ptr %slot)
  store ptr null, ptr %slot, align 8
  call goabiinternal void @consume_slice({ ptr, i64, i64 } %slice)
  br i1 %again, label %loop, label %exit

exit:
  ret void
}

attributes #0 = { "frame-pointer"="non-leaf" "go_results_tuple" }
attributes #1 = { "frame-pointer"="non-leaf" }

declare void @llvm.lifetime.start.p0(i64 immarg, ptr nocapture)
