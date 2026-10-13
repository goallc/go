target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-n8:16:32:64-S128"
; ERROR: goallc-lto: BSS declaration exceeds initialized definition: shared
@shared = global i64 4, section ".data", !goobj.symbol.nonpackage !0
!0 = !{i16 0}

!goobj.config = !{!900}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"definition-id", !"definition", !"0", !"0", !"0", !901, !"0200000000000000"}
!901 = !{}
