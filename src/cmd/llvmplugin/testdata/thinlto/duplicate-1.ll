; Duplicate native definitions must keep both package-indexed slots.
; OBJ: "class": "package"
; OBJ: "class_index": 0
; OBJ: "name": "shared"
; OBJ: "kind": "SDATA"
; IR: @shared = global i64 42
target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
@shared = global i64 42, !goobj.symbol.index !0
!0 = !{i32 0, i16 0}
!goobj.config = !{!1}
!1 = !{!"id1", !"pkg1", !"0", !"0", !"0100000000000000"}
!llvm.module.flags = !{!2}
!2 = !{i32 1, !"goobj.abi", !3}
!3 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !4}
!4 = !{}
