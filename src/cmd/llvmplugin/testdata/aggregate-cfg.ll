target triple = "x86_64-unknown-linux-goobj"

; IR-LABEL: define goabiinternal ptr @aggregate_diamond_call_skip(
; IR: i64 -4232994149196383034
; IR: phi ptr [ %value.leaf.0.relocated, %call ], [ %value.leaf.0, %skip ]
; IR: insertvalue %pair poison, ptr %value.leaf.0.relocated.merge

; IR-LABEL: define goabiinternal ptr @aggregate_branch_safepoints(
; IR-COUNT-2: @llvm.experimental.gc.statepoint
; IR: phi ptr [ %value.leaf.0.relocated{{[0-9]*}}, %left ], [ %value.leaf.0.relocated{{[0-9]*}}, %right ]

; IR-LABEL: define goabiinternal ptr @aggregate_sequential_conditional(
; IR: "gc-live"(ptr %value.leaf.0.relocated.merge
; IR: phi ptr

; IR-LABEL: define goabiinternal ptr @aggregate_natural_loop(
; IR: header:
; IR: phi ptr
; IR: "gc-live"(ptr %value.leaf.0.relocated.merge

; IR-LABEL: define goabiinternal ptr @aggregate_irreducible(
; IR-COUNT-3: = phi ptr
; IR: insertvalue %pair poison, ptr %value.leaf.0.relocated.merge

; IR-LABEL: define goabiinternal ptr @aggregate_phi_edge_use(
; IR: @llvm.experimental.gc.statepoint
; IR-COUNT-3: insertvalue %pair
; IR: %carried = phi %pair
; IR: @llvm.experimental.gc.statepoint
; IR: insertvalue %pair poison

; IR-LABEL: define goabiinternal ptr @aggregate_phi_duplicate_edge(
; IR: %[[PARTIAL:[-a-zA-Z$._0-9]+]] = insertvalue %pair poison
; IR: %[[REBUILT:[-a-zA-Z$._0-9]+]] = insertvalue %pair %[[PARTIAL]]
; IR: %carried = phi %pair [ %[[REBUILT]], %entry ], [ %[[REBUILT]], %entry ], [ %[[REBUILT]], %entry ]

; IR-LABEL: define goabiinternal ptr @aggregate_call_result_conditional(
; IR: call %pair @llvm.experimental.gc.result.{{[^(]+}}
; IR: = phi ptr
; IR: insertvalue %pair poison, ptr %value.leaf.0

; IR-LABEL: define goabiinternal ptr @aggregate_call_result_loop(
; IR: call %pair @llvm.experimental.gc.result.{{[^(]+}}
; IR: = phi ptr
; IR: insertvalue %pair poison, ptr %value.leaf.0

; IR-LABEL: define goabiinternal ptr @aggregate_call_result_irreducible(
; IR: call %pair @llvm.experimental.gc.result.{{[^(]+}}
; IR: = phi ptr
; IR: insertvalue %pair poison, ptr %value.leaf.0

; IR-LABEL: define goabiinternal ptr @aggregate_multiple_safepoints(
; IR: "gc-live"(ptr %value.leaf.0)
; IR: "gc-live"(ptr %value.leaf.0.relocated{{[0-9]*}})
; IR: "gc-live"(ptr %value.leaf.0.relocated{{[0-9]*}})

%pair = type { ptr, i64 }

declare goabiinternal void @safepoint()
declare goabiinternal %pair @make_pair(ptr, i64)
declare goabiinternal void @leaf_consume_pair(%pair) #0

define goabiinternal ptr @aggregate_diamond_call_skip(
    i1 %take_call, %pair %value)
    gc "goallc" {
entry:
  br i1 %take_call, label %call, label %skip

call:
  call goabiinternal void @safepoint()
  br label %merge

skip:
  br label %merge

merge:
  call goabiinternal void @leaf_consume_pair(%pair %value)
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal ptr @aggregate_branch_safepoints(
    i1 %take_left, %pair %value)
    gc "goallc" {
entry:
  br i1 %take_left, label %left, label %right

left:
  call goabiinternal void @safepoint()
  br label %merge

right:
  call goabiinternal void @safepoint()
  br label %merge

merge:
  call goabiinternal void @leaf_consume_pair(%pair %value)
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal ptr @aggregate_sequential_conditional(
    i1 %take_first, i1 %take_second, %pair %value)
    gc "goallc" {
entry:
  br i1 %take_first, label %first_call, label %first_skip

first_call:
  call goabiinternal void @safepoint()
  br label %first_merge

first_skip:
  br label %first_merge

first_merge:
  br i1 %take_second, label %second_call, label %second_skip

second_call:
  call goabiinternal void @safepoint()
  br label %second_merge

second_skip:
  br label %second_merge

second_merge:
  call goabiinternal void @leaf_consume_pair(%pair %value)
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal ptr @aggregate_natural_loop(
    i32 %count, %pair %value)
    gc "goallc" {
entry:
  br label %header

header:
  %index = phi i32 [ 0, %entry ], [ %next, %header ]
  call goabiinternal void @safepoint()
  %next = add nuw i32 %index, 1
  %again = icmp ult i32 %next, %count
  br i1 %again, label %header, label %exit

exit:
  call goabiinternal void @leaf_consume_pair(%pair %value)
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal ptr @aggregate_irreducible(
    i1 %enter_b, i1 %leave_a, i1 %leave_b, %pair %value)
    gc "goallc" {
entry:
  br i1 %enter_b, label %b, label %a

a:
  call goabiinternal void @safepoint()
  br i1 %leave_a, label %exit, label %b

b:
  br i1 %leave_b, label %exit, label %a

exit:
  call goabiinternal void @leaf_consume_pair(%pair %value)
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal ptr @aggregate_phi_edge_use(
    i1 %take_call, %pair %value)
    gc "goallc" {
entry:
  br i1 %take_call, label %call, label %skip

call:
  call goabiinternal void @safepoint()
  br label %merge

skip:
  br label %merge

merge:
  %carried = phi %pair [ %value, %call ], [ %value, %skip ]
  call goabiinternal void @safepoint()
  call goabiinternal void @leaf_consume_pair(%pair %carried)
  %result = extractvalue %pair %carried, 0
  ret ptr %result
}

define goabiinternal ptr @aggregate_phi_duplicate_edge(
    i32 %which, %pair %value)
    gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  switch i32 %which, label %merge [
    i32 0, label %merge
    i32 1, label %merge
  ]

merge:
  %carried = phi %pair [ %value, %entry ], [ %value, %entry ], [ %value, %entry ]
  call goabiinternal void @leaf_consume_pair(%pair %carried)
  %result = extractvalue %pair %carried, 0
  ret ptr %result
}

define goabiinternal ptr @aggregate_call_result_conditional(
    ptr %seed, i1 %take_call)
    gc "goallc" {
entry:
  %value = call goabiinternal %pair @make_pair(ptr %seed, i64 7)
  br i1 %take_call, label %call, label %skip

call:
  call goabiinternal void @safepoint()
  br label %merge

skip:
  br label %merge

merge:
  call goabiinternal void @leaf_consume_pair(%pair %value)
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal ptr @aggregate_call_result_loop(
    ptr %seed, i32 %count)
    gc "goallc" {
entry:
  %value = call goabiinternal %pair @make_pair(ptr %seed, i64 11)
  br label %header

header:
  %index = phi i32 [ 0, %entry ], [ %next, %header ]
  call goabiinternal void @safepoint()
  %next = add nuw i32 %index, 1
  %again = icmp ult i32 %next, %count
  br i1 %again, label %header, label %exit

exit:
  call goabiinternal void @leaf_consume_pair(%pair %value)
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal ptr @aggregate_call_result_irreducible(
    ptr %seed, i1 %enter_b, i1 %leave_a, i1 %leave_b)
    gc "goallc" {
entry:
  %value = call goabiinternal %pair @make_pair(ptr %seed, i64 13)
  br i1 %enter_b, label %b, label %a

a:
  call goabiinternal void @safepoint()
  br i1 %leave_a, label %exit, label %b

b:
  br i1 %leave_b, label %exit, label %a

exit:
  call goabiinternal void @leaf_consume_pair(%pair %value)
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal ptr @aggregate_multiple_safepoints(%pair %value)
    gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  call goabiinternal void @safepoint()
  call goabiinternal void @safepoint()
  call goabiinternal void @leaf_consume_pair(%pair %value)
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

attributes #0 = { "gc-leaf-function" }
