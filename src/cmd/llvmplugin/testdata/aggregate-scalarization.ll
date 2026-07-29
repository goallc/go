target triple = "x86_64-unknown-linux-goobj"

%pair = type { ptr, i64 }
%triple = type { i64, ptr, ptr }
%nested = type { i64, [2 x { ptr, i32 }] }

declare goabiinternal void @safepoint()
declare goabiinternal void @consume_pair(%pair)
declare goabiinternal %pair @make_pair(ptr, i64)
declare goabiinternal void @leaf_consume_pair(%pair) #0
declare goabiinternal void @leaf_consume_nested(%nested) #0

define goabiinternal ptr @pair_across_call(%pair %value) "go-stack-growth-statepoint" gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  %pointer = extractvalue %pair %value, 0
  ret ptr %pointer
}

define goabiinternal ptr @triple_across_call(%triple %value) "go-stack-growth-statepoint" gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  %first = extractvalue %triple %value, 1
  %second = extractvalue %triple %value, 2
  %same = icmp eq ptr %first, %second
  %result = select i1 %same, ptr %first, ptr %second
  ret ptr %result
}

define goabiinternal void @nested_across_call(%nested %value) "go-stack-growth-statepoint" gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  call goabiinternal void @leaf_consume_nested(%nested %value)
  ret void
}

define goabiinternal ptr @insertvalue_across_call(ptr %pointer, i64 %number) "go-stack-growth-statepoint" gc "goallc" {
entry:
  %with_pointer = insertvalue %pair zeroinitializer, ptr %pointer, 0
  %value = insertvalue %pair %with_pointer, i64 %number, 1
  call goabiinternal void @safepoint()
  call goabiinternal void @leaf_consume_pair(%pair %value)
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal ptr @phi_across_call(i1 %choose, %pair %left_value, %pair %right_value) "go-stack-growth-statepoint" gc "goallc" {
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

define goabiinternal ptr @select_across_call(i1 %choose, %pair %left_value, %pair %right_value) "go-stack-growth-statepoint" gc "goallc" {
entry:
  %value = select i1 %choose, %pair %left_value, %pair %right_value
  call goabiinternal void @safepoint()
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal ptr @phi_edge_use(%pair %value) "go-stack-growth-statepoint" gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  br label %merge

merge:
  %carried = phi %pair [ %value, %entry ]
  %result = extractvalue %pair %carried, 0
  ret ptr %result
}

define goabiinternal ptr @multiple_calls(%pair %value) "go-stack-growth-statepoint" gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  call goabiinternal void @safepoint()
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal ptr @aggregate_call_result(ptr %pointer) "go-stack-growth-statepoint" gc "goallc" {
entry:
  %value = call goabiinternal %pair @make_pair(ptr %pointer, i64 7)
  call goabiinternal void @safepoint()
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal void @aggregate_current_call_argument(%pair %value) "go-stack-growth-statepoint" gc "goallc" {
entry:
  call goabiinternal void @consume_pair(%pair %value)
  ret void
}

define goabiinternal void @aggregate_load_store(ptr %source, ptr %destination) "go-stack-growth-statepoint" gc "goallc" {
entry:
  %value = load %pair, ptr %source, align 8
  call goabiinternal void @safepoint()
  store %pair %value, ptr %destination, align 8
  ret void
}

define goabiinternal ptr @alloca_derived_leaf(i64 %number) "go-stack-growth-statepoint" gc "goallc" {
entry:
  %slot = alloca i64, align 8
  store i64 %number, ptr %slot, align 8
  %value = insertvalue %pair poison, ptr %slot, 0
  call goabiinternal void @safepoint()
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

define goabiinternal void @frozen_aggregate() "go-stack-growth-statepoint" gc "goallc" {
entry:
  %value = freeze %pair poison
  call goabiinternal void @safepoint()
  call goabiinternal void @leaf_consume_pair(%pair %value)
  ret void
}

attributes #0 = { "gc-leaf-function" }
