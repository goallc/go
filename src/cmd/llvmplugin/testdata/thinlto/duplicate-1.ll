; Duplicate native definitions must keep both package-indexed slots.
; OBJ: "class": "package"
; OBJ: "class_index": 0
; OBJ: "name": "shared"
; OBJ: "kind": "SDATA"
; IR: @shared = global i64 42
; IR-LABEL: define {{.*}}@pkg1.address(
; IR: call goabiinternal ptr @pkg0.address()
target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-v128:64:64-v256:64:64-v512:64:64-n8:16:32:64-S64"
@shared = global i64 42, !goobj.symbol.index !0
!0 = !{i32 0, i16 0}
!goobj.config = !{!1}
!1 = !{!"id1", !"pkg1", !"0", !"0", !"0100000000000000"}
!llvm.module.flags = !{!2}
!2 = !{i32 1, !"goobj.abi", !3}
!3 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !4}
!4 = !{}

; Importing pkg0.address would bind its reference to the wrong package slot.
define goabiinternal ptr @pkg1.address() !goobj.symbol.index !5 {
  %p = call goabiinternal ptr @pkg0.address()
  ret ptr %p
}
declare !goobj.import !6 goabiinternal ptr @pkg0.address()
!5 = !{i32 1, i16 1}
!6 = !{!"pkg0", i32 1, i32 0}
declare goabi0 void @"runtime.morestack_noctxt<builtin.246><ABI0>"()
@llvm.compiler.used = appending global [1 x ptr] [ptr @"runtime.morestack_noctxt<builtin.246><ABI0>"], section "llvm.metadata"
