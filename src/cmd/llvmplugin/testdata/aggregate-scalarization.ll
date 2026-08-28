target triple = "x86_64-unknown-linux-goobj"

; IR-LABEL: define goabiinternal ptr @pair_across_call(
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %value.leaf.0)

; IR-LABEL: define goabiinternal ptr @triple_across_call(
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %value.leaf.1, ptr %value.leaf.2)

; IR-LABEL: define goabiinternal void @nested_across_call(
; IR: extractvalue %nested %value, 1, 1, 0

; IR-LABEL: define goabiinternal ptr @fixed_vector_across_call(
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(<2 x ptr> %value)
; IR: call coldcc <2 x ptr> @llvm.experimental.gc.relocate.v2p0

; IR-LABEL: define goabiinternal ptr @nested_fixed_vector_across_call(
; IR: %[[VECTOR_LEAF:[-a-zA-Z$._0-9]+]] = extractvalue %vector_pair %value, 0
; IR: "gc-live"(<2 x ptr> %[[VECTOR_LEAF]])
; IR: %[[VECTOR_RELOCATED:[-a-zA-Z$._0-9]+]] = call coldcc <2 x ptr> @llvm.experimental.gc.relocate.v2p0
; IR: %value.relocated = insertvalue %vector_pair %value, <2 x ptr> %[[VECTOR_RELOCATED]], 0

; IR-LABEL: define goabiinternal ptr @insertvalue_across_call(
; IR: %[[INSERTED_LEAF:[-a-zA-Z$._0-9]+]] = extractvalue %pair %value, 0
; IR: "gc-live"(ptr %[[INSERTED_LEAF]])
; IR: %[[INSERTED_RELOCATED:[-a-zA-Z$._0-9]+]] = call coldcc ptr @llvm.experimental.gc.relocate.p0
; IR: %value.relocated = insertvalue %pair %value, ptr %[[INSERTED_RELOCATED]], 0

; IR-LABEL: define goabiinternal ptr @inserted_pointer_also_live(
; IR-NOT: %value.leaf.0 = extractvalue
; IR: "gc-live"(ptr %pointer)
; IR: %pointer.relocated = call coldcc ptr @llvm.experimental.gc.relocate.p0
; IR: %value.relocated = insertvalue %pair %value, ptr %pointer.relocated, 0
; IR: ret ptr %pointer.relocated

; IR-LABEL: define goabiinternal ptr @extracted_pointer_also_live(
; IR-NOT: %value.leaf.0 = extractvalue
; IR: "gc-live"(ptr %pointer)
; IR-COUNT-1: call coldcc ptr @llvm.experimental.gc.relocate.p0
; IR: %value.relocated = insertvalue %pair %value, ptr %pointer.relocated, 0
; IR: ret ptr %pointer.relocated

; IR-LABEL: define goabiinternal ptr @inserted_call_result_also_live(
; IR: [[CALL_RESULT:%.*]] = call ptr @llvm.experimental.gc.result.p0
; IR-NOT: extractvalue %pair
; IR: "gc-live"(ptr [[CALL_RESULT]])
; IR: [[RELOCATED:%.*]] = call coldcc ptr @llvm.experimental.gc.relocate.p0
; IR: insertvalue %pair %value, ptr [[RELOCATED]], 0
; IR: ret ptr [[RELOCATED]]

; IR-LABEL: define goabiinternal ptr @inserted_pointer_in_call_argument(
; IR: @llvm.experimental.gc.statepoint{{.*}}@consume_slice{{.*}}{ ptr, i64, i64 } %slice{{.*}}"gc-live"(ptr %pointer)
; IR-COUNT-1: call coldcc ptr @llvm.experimental.gc.relocate.p0
; IR: insertvalue %pair %value, ptr %pointer.relocated, 0
; IR: ret ptr %pointer.relocated

; IR-LABEL: define goabiinternal ptr @inserted_leaf_in_call_argument(
; IR: @llvm.experimental.gc.statepoint{{.*}}@consume_pair{{.*}}%pair %value{{.*}}"gc-live"(ptr %pointer)
; IR-COUNT-1: call coldcc ptr @llvm.experimental.gc.relocate.p0
; IR: insertvalue %pair %value, ptr %pointer.relocated, 0
; IR: ret ptr %pointer.relocated

; IR-LABEL: define goabiinternal ptr @inserted_call_result_leaf_in_call_argument(
; IR: [[CALL_RESULT:%.*]] = call ptr @llvm.experimental.gc.result.p0
; IR: @llvm.experimental.gc.statepoint{{.*}}@consume_pair{{.*}}%pair %value{{.*}}"gc-live"(ptr [[CALL_RESULT]])
; IR-COUNT-1: call coldcc ptr @llvm.experimental.gc.relocate.p0
; IR: insertvalue %pair %value, ptr %pointer.relocated, 0
; IR: ret ptr %pointer.relocated

; IR-LABEL: define goabiinternal ptr @inserted_pointer_used_by_derived(
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %pointer)
; IR-COUNT-1: call coldcc ptr @llvm.experimental.gc.relocate.p0
; IR: %derived.remat = getelementptr i8, ptr %pointer.relocated, i64 8
; IR: insertvalue %pair %value, ptr %pointer.relocated, 0

; IR-LABEL: define goabiinternal ptr @partial_insertvalue_across_call(
; IR-NOT: %partial.leaf.1
; IR: %[[PARTIAL_LEAF:[-a-zA-Z$._0-9]+]] = extractvalue %reflect_value %partial, 0
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %[[PARTIAL_LEAF]])
; IR: %[[PARTIAL_RELOCATED:[-a-zA-Z$._0-9]+]] = call coldcc ptr @llvm.experimental.gc.relocate.p0
; IR: %partial.relocated = insertvalue %reflect_value %partial, ptr %[[PARTIAL_RELOCATED]], 0

; IR-LABEL: define goabiinternal ptr @phi_across_call(
; IR: %value.leaf.0 = extractvalue %pair %value, 0

; IR-LABEL: define goabiinternal ptr @multiple_calls(
; IR: %[[FIRST_LEAF:[-a-zA-Z$._0-9]+]] = extractvalue %pair %value, 0
; IR: "gc-live"(ptr %[[FIRST_LEAF]])
; IR: %[[FIRST_RELOCATED:[-a-zA-Z$._0-9]+]] = call coldcc ptr @llvm.experimental.gc.relocate.p0
; IR: %[[FIRST_AGGREGATE:[-a-zA-Z$._0-9]+]] = insertvalue %pair %value, ptr %[[FIRST_RELOCATED]], 0
; IR: %[[SECOND_LEAF:[-a-zA-Z$._0-9]+]] = extractvalue %pair %[[FIRST_AGGREGATE]], 0
; IR: "gc-live"(ptr %[[SECOND_LEAF]])
; IR: %[[SECOND_RELOCATED:[-a-zA-Z$._0-9]+]] = call coldcc ptr @llvm.experimental.gc.relocate.p0
; IR: insertvalue %pair %[[FIRST_AGGREGATE]], ptr %[[SECOND_RELOCATED]], 0

; IR-LABEL: define goabiinternal ptr @aggregate_call_result(
; IR: call %pair @llvm.experimental.gc.result.{{[^(]+}}

; IR-LABEL: define goabiinternal void @aggregate_current_call_argument(
; IR-NOT: extractvalue
; IR-NOT: "gc-live"
; IR: @llvm.experimental.gc.statepoint{{.*}}@consume_pair{{.*}}%pair %value
; IR: ret void

; IR-LABEL: define goabiinternal void @aggregate_load_store(
; IR: %value.leaf.0 = extractvalue %pair %value, 0

; IR-LABEL: define goabiinternal void @frozen_aggregate(
; IR: %value = freeze %pair poison

%pair = type { ptr, i64 }
%triple = type { i64, ptr, ptr }
%reflect_value = type { ptr, ptr, i64 }
%nested = type { i64, [2 x { ptr, i32 }] }
%vector_pair = type { <2 x ptr>, i64 }

declare goabiinternal void @safepoint()
declare goabiinternal void @consume_pair(%pair)
declare goabiinternal %pair @make_pair(ptr, i64)
declare goabiinternal ptr @make_pointer()
declare goabiinternal void @consume_slice({ ptr, i64, i64 })
declare goabiinternal void @leaf_consume_pair(%pair) #0
declare goabiinternal void @leaf_consume_nested(%nested) #0
declare goabiinternal void @leaf_consume_vector_pair(%vector_pair) #0

define goabiinternal ptr @pair_across_call(%pair %value) gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  %pointer = extractvalue %pair %value, 0
  ret ptr %pointer
}

define goabiinternal ptr @triple_across_call(%triple %value) gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  %first = extractvalue %triple %value, 1
  %second = extractvalue %triple %value, 2
  %same = icmp eq ptr %first, %second
  %result = select i1 %same, ptr %first, ptr %second
  ret ptr %result
}

define goabiinternal void @nested_across_call(%nested %value) gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  call goabiinternal void @leaf_consume_nested(%nested %value)
  ret void
}

define goabiinternal ptr @fixed_vector_across_call(ptr %source) gc "goallc" {
entry:
  %value = load <2 x ptr>, ptr %source, align 8
  call goabiinternal void @safepoint()
  %result = extractelement <2 x ptr> %value, i32 1
  ret ptr %result
}

define goabiinternal ptr @nested_fixed_vector_across_call(ptr %source, i64 %number) gc "goallc" {
entry:
  %vector.value = load <2 x ptr>, ptr %source, align 8
  %with_vector = insertvalue %vector_pair poison, <2 x ptr> %vector.value, 0
  %value = insertvalue %vector_pair %with_vector, i64 %number, 1
  call goabiinternal void @safepoint()
  call goabiinternal void @leaf_consume_vector_pair(%vector_pair %value)
  %vector = extractvalue %vector_pair %value, 0
  %result = extractelement <2 x ptr> %vector, i32 1
  ret ptr %result
}

define goabiinternal ptr @insertvalue_across_call(ptr %pointer, i64 %number) gc "goallc" {
entry:
  %with_pointer = insertvalue %pair zeroinitializer, ptr %pointer, 0
  %value = insertvalue %pair %with_pointer, i64 %number, 1
  call goabiinternal void @safepoint()
  call goabiinternal void @leaf_consume_pair(%pair %value)
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

; A pointer inserted into an aggregate and independently live after the same
; safepoint remains one SSA root. The relocated pointer updates both uses.
define goabiinternal ptr @inserted_pointer_also_live(ptr %pointer, i64 %number) gc "goallc" {
entry:
  %with_pointer = insertvalue %pair zeroinitializer, ptr %pointer, 0
  %value = insertvalue %pair %with_pointer, i64 %number, 1
  call goabiinternal void @safepoint()
  call goabiinternal void @leaf_consume_pair(%pair %value)
  ret ptr %pointer
}

; Reuse an existing dominating projection of an opaque aggregate definition
; instead of creating a second extractvalue root for the same leaf.
define goabiinternal ptr @extracted_pointer_also_live(%pair %value) gc "goallc" {
entry:
  %pointer = extractvalue %pair %value, 0
  call goabiinternal void @safepoint()
  call goabiinternal void @leaf_consume_pair(%pair %value)
  ret ptr %pointer
}

; The directly inserted pointer is also live as a scalar. Whole-value
; liveness lets both uses share the same SSA root even when that root is the
; result of an earlier statepoint call.
define goabiinternal ptr @inserted_call_result_also_live(i64 %number) gc "goallc" {
entry:
  %pointer = call goabiinternal ptr @make_pointer()
  %with_pointer = insertvalue %pair zeroinitializer, ptr %pointer, 0
  %value = insertvalue %pair %with_pointer, i64 %number, 1
  call goabiinternal void @safepoint()
  call goabiinternal void @leaf_consume_pair(%pair %value)
  ret ptr %pointer
}

; A pointer can simultaneously supply a current call argument, a live
; aggregate field, and an independently live scalar without becoming multiple
; gc-live roots.
define goabiinternal ptr @inserted_pointer_in_call_argument(ptr %pointer, i64 %number) gc "goallc" {
entry:
  %with_pointer = insertvalue %pair zeroinitializer, ptr %pointer, 0
  %value = insertvalue %pair %with_pointer, i64 %number, 1
  %slice_with_pointer = insertvalue { ptr, i64, i64 } poison, ptr %pointer, 0
  %slice_with_length = insertvalue { ptr, i64, i64 } %slice_with_pointer, i64 1, 1
  %slice = insertvalue { ptr, i64, i64 } %slice_with_length, i64 1, 2
  call goabiinternal void @consume_slice({ ptr, i64, i64 } %slice)
  call goabiinternal void @leaf_consume_pair(%pair %value)
  ret ptr %pointer
}

define goabiinternal ptr @inserted_leaf_in_call_argument(ptr %pointer, i64 %number) gc "goallc" {
entry:
  %with_pointer = insertvalue %pair zeroinitializer, ptr %pointer, 0
  %value = insertvalue %pair %with_pointer, i64 %number, 1
  call goabiinternal void @consume_pair(%pair %value)
  call goabiinternal void @leaf_consume_pair(%pair %value)
  ret ptr %pointer
}

define goabiinternal ptr @inserted_call_result_leaf_in_call_argument(i64 %number) gc "goallc" {
entry:
  %pointer = call goabiinternal ptr @make_pointer()
  %with_pointer = insertvalue %pair zeroinitializer, ptr %pointer, 0
  %value = insertvalue %pair %with_pointer, i64 %number, 1
  call goabiinternal void @consume_pair(%pair %value)
  call goabiinternal void @leaf_consume_pair(%pair %value)
  ret ptr %pointer
}

; Derived-address repair and aggregate rebuilding also share the same base
; pointer relocation.
define goabiinternal ptr @inserted_pointer_used_by_derived(ptr %pointer, i64 %number) gc "goallc" {
entry:
  %with_pointer = insertvalue %pair zeroinitializer, ptr %pointer, 0
  %value = insertvalue %pair %with_pointer, i64 %number, 1
  %derived = getelementptr i8, ptr %pointer, i64 8
  call goabiinternal void @safepoint()
  call goabiinternal void @leaf_consume_pair(%pair %value)
  %result = load ptr, ptr %derived
  ret ptr %result
}

; Relocating a partially initialized aggregate must not manufacture SSA roots
; for poison pointer leaves. The original aggregate base preserves such leaves
; until they are overwritten without being observed, as happens while
; reflect.Value is assembled for a later call.
define goabiinternal ptr @partial_insertvalue_across_call(ptr %pointer, i64 %number) gc "goallc" {
entry:
  %partial = insertvalue %reflect_value poison, ptr %pointer, 0
  call goabiinternal void @safepoint()
  %with_data = insertvalue %reflect_value %partial, ptr @partial_insertvalue_across_call, 1
  %value = insertvalue %reflect_value %with_data, i64 %number, 2
  %leaf = extractvalue %reflect_value %value, 0
  ret ptr %leaf
}

define goabiinternal ptr @phi_across_call(i1 %choose, %pair %left_value, %pair %right_value) gc "goallc" {
entry:
  br i1 %choose, label %left, label %right

left:
  br label %merge

right:
  br label %merge

merge:
  %value = phi %pair [ %left_value, %left ], [ %right_value, %right ]
  call goabiinternal void @safepoint()
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal ptr @select_across_call(i1 %choose, %pair %left_value, %pair %right_value) gc "goallc" {
entry:
  %value = select i1 %choose, %pair %left_value, %pair %right_value
  call goabiinternal void @safepoint()
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal ptr @phi_edge_use(%pair %value) gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  br label %merge

merge:
  %carried = phi %pair [ %value, %entry ]
  %result = extractvalue %pair %carried, 0
  ret ptr %result
}

define goabiinternal ptr @multiple_calls(%pair %value) gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  call goabiinternal void @safepoint()
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal ptr @aggregate_call_result(ptr %pointer) gc "goallc" {
entry:
  %value = call goabiinternal %pair @make_pair(ptr %pointer, i64 7)
  call goabiinternal void @safepoint()
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal void @aggregate_current_call_argument(%pair %value) gc "goallc" {
entry:
  call goabiinternal void @consume_pair(%pair %value)
  ret void
}

define goabiinternal void @aggregate_load_store(ptr %source, ptr %destination) gc "goallc" {
entry:
  %value = load %pair, ptr %source, align 8
  call goabiinternal void @safepoint()
  store %pair %value, ptr %destination, align 8
  ret void
}

define goabiinternal ptr @alloca_derived_leaf(i64 %number) gc "goallc" {
entry:
  %slot = alloca i64, align 8
  store i64 %number, ptr %slot, align 8
  %value = insertvalue %pair poison, ptr %slot, 0
  call goabiinternal void @safepoint()
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal void @frozen_aggregate() gc "goallc" {
entry:
  %value = freeze %pair poison
  call goabiinternal void @safepoint()
  call goabiinternal void @leaf_consume_pair(%pair %value)
  ret void
}

attributes #0 = { "gc-leaf-function" }
