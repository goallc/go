target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-n8:16:32:64-S128"
; CHECK: @shared = external global [2 x i64]
; CHECK-NOT: @shared = global
@shared = global [2 x i64] zeroinitializer, section ".bss", !goobj.symbol.nonpackage !0
!0 = !{i16 0}

!goobj.config = !{!900}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"declaration-id", !"declaration", !"0", !"0", !"0", !901, !"0100000000000000"}
!901 = !{}
