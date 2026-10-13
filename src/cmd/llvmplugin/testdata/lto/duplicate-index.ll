target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-n8:16:32:64-S128"
; ERROR: goallc-lto: duplicate package symbol index:
@first = global i64 4, !goobj.symbol.index !0
@second = global i64 5, !goobj.symbol.index !0
!0 = !{i32 0}

!goobj.config = !{!900}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"duplicate-id", !"duplicate", !"0", !"0", !"0", !901, !"0100000000000000"}
!901 = !{}
