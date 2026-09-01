target triple = "aarch64-unknown-linux-gnu"

; CHECK-LABEL: define goabi0 void @"abi0_tail<ABI0>"
; CHECK: musttail call goabiinternal void @target()
; CHECK-NEXT: ret void
; CHECK-NOT: llvm.experimental.gc.statepoint
; CHECK-LABEL: define goabiinternal void @fmv_terminal_tail()
; CHECK: musttail call goabiinternal void @target()
; CHECK-NEXT: ret void
; CHECK-NOT: llvm.experimental.gc.statepoint
; CHECK-LABEL: define goabiinternal ptr @fmv_terminal_tail_result(
; CHECK: %result = musttail call goabiinternal ptr @pointer_target(ptr %p)
; CHECK-NEXT: ret ptr %result
; CHECK-NOT: llvm.experimental.gc.statepoint
; CHECK-LABEL: define goabiinternal void @ordinary_terminal_tail()
; CHECK: call goabiinternal token {{.*}}@llvm.experimental.gc.statepoint
; CHECK-LABEL: define goabiinternal ptr @nonterminal_tail(
; CHECK: call goabiinternal token {{.*}}@llvm.experimental.gc.statepoint
; CHECK: call coldcc ptr @llvm.experimental.gc.relocate

declare goabiinternal void @target()
declare goabiinternal ptr @pointer_target(ptr)

define goabi0 void @"abi0_tail<ABI0>"() "gc-leaf-function" "go-nosplit" gc "goallc" {
entry:
  musttail call goabiinternal void @target()
  ret void
}

define goabiinternal void @fmv_terminal_tail() gc "goallc" {
entry:
  tail call goabiinternal void @target(), !goallc.cpu.tail_transfer !0
  ret void
}

define goabiinternal ptr @fmv_terminal_tail_result(ptr %p) gc "goallc" {
entry:
  %result = tail call goabiinternal ptr @pointer_target(ptr %p), !goallc.cpu.tail_transfer !0
  ret ptr %result
}

define goabiinternal void @ordinary_terminal_tail() gc "goallc" {
entry:
  tail call goabiinternal void @target()
  ret void
}

define goabiinternal ptr @nonterminal_tail(ptr %p) gc "goallc" {
entry:
  tail call goabiinternal void @target(), !goallc.cpu.tail_transfer !0
  %next = getelementptr i8, ptr %p, i64 1
  ret ptr %next
}

!0 = !{}
