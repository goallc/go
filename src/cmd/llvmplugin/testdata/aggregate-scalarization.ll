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
; IR: %value.leaf.0 = extractvalue %vector_pair %value, 0
; IR: insertvalue %vector_pair {{.*}}<2 x ptr> %value.leaf.0.relocated, 0

; IR-LABEL: define goabiinternal ptr @insertvalue_across_call(
; IR: insertvalue %pair {{.*}}ptr %value.leaf.0.relocated

; IR-LABEL: define goabiinternal ptr @partial_insertvalue_across_call(
; IR: %partial.leaf.0 = extractvalue %reflect_value %partial, 0
; IR-NOT: %partial.leaf.1
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %partial.leaf.0)

; IR-LABEL: define goabiinternal ptr @phi_across_call(
; IR: %value.leaf.0 = extractvalue %pair %value, 0

; IR-LABEL: define goabiinternal ptr @multiple_calls(
; IR: %value.leaf.0.relocated2

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

; Scalarizing a partially initialized aggregate must not manufacture SSA
; pointer roots for poison leaves. Those leaves can be overwritten after the
; safepoint without ever being observed, as happens while reflect.Value is
; assembled for a later call.
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
