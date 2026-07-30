target triple = "x86_64-unknown-linux-goobj"

%nested = type { ptr, i64, [2 x { i32, ptr }] }
%pointer_field = type { i64, ptr }

declare goabiinternal void @safepoint()
declare goabiinternal void @mutate_pointer_slot(ptr)

define goabiinternal ptr @pointer_slot(ptr %pointer) "go-stack-growth-statepoint" gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %pointer, ptr %slot, align 8
  %nilcheck = load volatile i8, ptr %slot, align 1, !goallc.nilcheck !0
  call goabiinternal void @safepoint()
  %result = load ptr, ptr %slot, align 8
  ret ptr %result
}

define goabiinternal ptr @nested_whole_aggregate(
    ptr %first, ptr %second, ptr %third) "go-stack-growth-statepoint" gc "goallc" {
entry:
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
    i1 %take_call, ptr %pointer) "go-stack-growth-statepoint" gc "goallc" {
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
    ptr %pointer) "go-stack-growth-statepoint" gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %pointer, ptr %slot, align 8
  call goabiinternal void @safepoint()
  call goabiinternal void @safepoint()
  %result = load ptr, ptr %slot, align 8
  ret ptr %result
}

define goabiinternal ptr @alloca_loop(
    i1 %again, ptr %pointer) "go-stack-growth-statepoint" gc "goallc" {
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
    ptr %pointer) "go-stack-growth-statepoint" gc "goallc" {
entry:
  %slot = alloca %pointer_field, align 8
  %field = getelementptr inbounds %pointer_field, ptr %slot, i32 0, i32 1
  store ptr %pointer, ptr %field, align 8
  call goabiinternal void @safepoint()
  %result = load ptr, ptr %field, align 8
  ret ptr %result
}

define goabiinternal ptr @alloca_address_passed_to_callee(
    ptr %pointer) "go-stack-growth-statepoint" gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %pointer, ptr %slot, align 8
  call goabiinternal void @mutate_pointer_slot(ptr %slot)
  %result = load ptr, ptr %slot, align 8
  ret ptr %result
}

define goabiinternal void @alloca_uninitialized_at_safepoint(
    ptr %pointer) "go-stack-growth-statepoint" gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  call goabiinternal void @safepoint()
  store ptr %pointer, ptr %slot, align 8
  ret void
}

!0 = !{}
