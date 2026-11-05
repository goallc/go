target triple = "x86_64-unknown-linux-goobj"

%pair = type { ptr, i64 }

declare goabiinternal void @safepoint()

; IR-LABEL: define goabiinternal ptr @aggregate_loop_relocation(
; IR: %value.leaf.0.statepoint.home = alloca ptr
; IR: store ptr %value.leaf.0, ptr %value.leaf.0.statepoint.home
; IR: header:
; IR-NOT: store ptr
; IR: call:
; IR: @llvm.experimental.gc.statepoint
; IR-SAME: "gc-live"(ptr %value.leaf.0.statepoint.home)
; IR-NOT: @llvm.experimental.gc.relocate
; IR: exit:
; IR: %[[RESULT:.*]] = load volatile ptr, ptr %value.leaf.0.statepoint.home
; IR: ret ptr %[[RESULT]]
define goabiinternal ptr @aggregate_loop_relocation(i1 %take_call, i1 %again, %pair %value) gc "goallc" {
entry:
  br label %header

header:
  br i1 %take_call, label %call, label %skip

call:
  call goabiinternal void @safepoint()
  br label %latch

skip:
  br label %latch

latch:
  br i1 %again, label %header, label %exit

exit:
  %result = extractvalue %pair %value, 0
  ret ptr %result
}
