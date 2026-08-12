target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @callee()
declare goabiinternal void @leaf_callee() #0
declare goabiinternal ptr @make_pointer()

define goabiinternal i64 @pointer_live_across_call(ptr %p) gc "goallc" {
entry:
  call goabiinternal void @callee()
  %value = load i64, ptr %p, align 8
  ret i64 %value
}

define goabiinternal i64 @stack_address_live_across_call() gc "goallc" {
entry:
  %slot = alloca i64, align 8
  store i64 42, ptr %slot, align 8
  call goabiinternal void @callee()
  %value = load i64, ptr %slot, align 8
  ret i64 %value
}

define goabiinternal void @explicit_leaf_call() gc "goallc" {
entry:
  call goabiinternal void @leaf_callee()
  ret void
}

define goabiinternal i64 @pointer_live_across_two_calls(ptr %p) gc "goallc" {
entry:
  call goabiinternal void @callee()
  call goabiinternal void @callee()
  %value = load i64, ptr %p, align 8
  ret i64 %value
}

define goabiinternal i64 @pointer_live_into_cfg(ptr %p, i1 %take_left) gc "goallc" {
entry:
  call goabiinternal void @callee()
  br i1 %take_left, label %left, label %right

left:
  %left_value = load i64, ptr %p, align 8
  br label %done

right:
  %right_value = load i64, ptr %p, align 8
  br label %done

done:
  %value = phi i64 [ %left_value, %left ], [ %right_value, %right ]
  ret i64 %value
}

define goabiinternal ptr @call_result_live_across_call() gc "goallc" {
entry:
  %pointer = call goabiinternal ptr @make_pointer()
  call goabiinternal void @callee()
  ret ptr %pointer
}

; Function block layout is intentionally different from CFG dominance. The
; safepoint in %use is visited before the pointer-producing call in %define,
; but its liveness set contains that call result.
define goabiinternal ptr @out_of_layout_call_result() gc "goallc" {
entry:
  br label %define

use:
  call goabiinternal void @callee()
  ret ptr %pointer

define:
  %pointer = call goabiinternal ptr @make_pointer()
  br label %use
}

attributes #0 = { "gc-leaf-function" }
