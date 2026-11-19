; Duplicate native definitions must keep both package-indexed slots.
; OBJ: "class": "package"
; OBJ: "class_index": 0
; OBJ: "name": "shared"
; OBJ: "kind": "SBSS"
; IR: @"__goid$v1$706B6730$p0$shared" = global i64 0
target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-v128:64:64-v256:64:64-v512:64:64-n8:16:32:64-S64"
@shared = global i64 0, !goobj.symbol.index !0
!0 = !{i32 0, i16 0}
!goobj.config = !{!1}
!1 = !{!"id0", !"pkg0", !"0", !"0", !"0100000000000000"}
!llvm.module.flags = !{!2}
!2 = !{i32 1, !"goobj.abi", !3}
!3 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !4}
!4 = !{}

; The nested constant must retain its package identity when imported.
define goabiinternal ptr @pkg0.address() alwaysinline !goobj.symbol.index !5 {
  ret ptr getelementptr (i8, ptr @shared, i64 1)
}
!5 = !{i32 1, i16 1}
