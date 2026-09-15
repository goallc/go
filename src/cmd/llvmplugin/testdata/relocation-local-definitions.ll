target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal ptr @make_pointer()
declare goabiinternal void @callee()

; A call result is a fresh definition in a continuation block. Uses before
; the next safepoint must use it; later uses must use its relocation.
; IR-LABEL: define goabiinternal i8 @local_call_result(
; IR: [[P:%[^ ]+]] = call ptr @llvm.experimental.gc.result
; IR: %before = load i8, ptr [[P]]
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr [[P]])
; IR: [[R:%[^ ]+]] = call {{.*}}ptr @llvm.experimental.gc.relocate
; IR: %after = load i8, ptr [[R]]
define goabiinternal i8 @local_call_result() gc "goallc" {
entry:
  %p = call goabiinternal ptr @make_pointer()
  %before = load i8, ptr %p
  call goabiinternal void @callee()
  %after = load i8, ptr %p
  %sum = add i8 %before, %after
  ret i8 %sum
}

; Each iteration defines p again, overriding the previous iteration's
; relocation. This also exercises an existing PHI's self-edge use of slot.
; IR-LABEL: define goabiinternal i8 @loop_local_definition(
; IR: %p = load ptr, ptr {{.*}}
; IR: %before = load i8, ptr %p
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"({{.*}}ptr %p
; IR: [[P_RELOC:%p.relocated[^ ]*]] = call {{.*}}ptr @llvm.experimental.gc.relocate
; IR: %after = load i8, ptr [[P_RELOC]]
define goabiinternal i8 @loop_local_definition(ptr %slot, i1 %again)
    gc "goallc" {
entry:
  br label %loop
loop:
  %current = phi ptr [ %slot, %entry ], [ %slot, %loop ]
  %p = load ptr, ptr %current
  %before = load i8, ptr %p
  call goabiinternal void @callee()
  %after = load i8, ptr %p
  %sum = add i8 %before, %after
  br i1 %again, label %loop, label %exit
exit:
  ret i8 %sum
}
