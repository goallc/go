target triple = "x86_64-unknown-linux-goobj"

; IR-LABEL: define goabiinternal void @indirect_callee(
; IR: @llvm.experimental.gc.statepoint
; IR-NOT: "gc-live"
; IR-NOT: @llvm.experimental.gc.relocate
; IR: ret void
; IR-LABEL: define goabiinternal void @call_only_pointer_argument(
; IR: @llvm.experimental.gc.statepoint
; IR-NOT: "gc-live"
; IR-NOT: @llvm.experimental.gc.relocate
; IR: ret void

; X86-OBJVIEW-LABEL: TEXT indirect_callee(SB)
; X86-OBJVIEW: CALL {{.*}} [0:0]R_CALLIND
; X86-OBJVIEW: R_CALL:runtime.morestack_noctxt
; X86-OBJVIEW-LABEL: TEXT memory_indirect_callee(SB)
; X86-OBJVIEW: CALL {{.*}} [0:0]R_CALLIND
; X86-OBJVIEW: R_CALL:runtime.morestack_noctxt
; X86-OBJVIEW-LABEL: TEXT call_only_pointer_argument(SB)
; X86-OBJVIEW-NOT: R_CALLIND
; X86-OBJVIEW: R_CALL:consume_pointer
; X86-OBJVIEW: R_CALL:runtime.morestack_noctxt

; AArch64-OBJVIEW-LABEL: TEXT indirect_callee(SB)
; AArch64-OBJVIEW: R_CALLARM64:runtime.morestack_noctxt
; AArch64-OBJVIEW: CALL {{.*}} [0:0]R_CALLIND
; AArch64-OBJVIEW-LABEL: TEXT memory_indirect_callee(SB)
; AArch64-OBJVIEW: R_CALLARM64:runtime.morestack_noctxt
; AArch64-OBJVIEW: CALL {{.*}} [0:0]R_CALLIND
; AArch64-OBJVIEW-LABEL: TEXT call_only_pointer_argument(SB)
; AArch64-OBJVIEW-NOT: R_CALLIND
; AArch64-OBJVIEW: R_CALLARM64:runtime.morestack_noctxt
; AArch64-OBJVIEW: R_CALLARM64:consume_pointer

declare goabiinternal void @consume_pointer(ptr)

define goabiinternal void @indirect_callee(ptr %callee, i64 %arg) #0 gc "goallc" {
entry:
  call goabiinternal void %callee(i64 %arg)
  ret void
}

define goabiinternal void @memory_indirect_callee(ptr %callee_slot, i64 %arg) #0 gc "goallc" {
entry:
  %callee = load ptr, ptr %callee_slot, align 8
  call goabiinternal void %callee(i64 %arg)
  ret void
}

define goabiinternal void @call_only_pointer_argument(ptr %value) #0 gc "goallc" {
entry:
  call goabiinternal void @consume_pointer(ptr %value)
  ret void
}

attributes #0 = { "frame-pointer"="non-leaf" }
