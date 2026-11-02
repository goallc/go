; CHECK: @shared = {{.*}}global [4 x i64] [i64 1, i64 2, i64 3, i64 4], section ".data"
; CHECK-SAME: !goobj.symbol.index ![[INDEX:[0-9]+]]
; CHECK: ![[INDEX]] = !{i32 0, i16 0}
; TYPE: @shared = {{.*}}!goobj.gotype ![[TYPE:[0-9]+]]
; TYPE: ![[TYPE]] = !{ptr @type.initialized}
; FORWARD: "index":0,"name":"shared","originals":[{"index":7,"package":0}],"package":1
; REVERSE: "index":0,"name":"shared","originals":[{"index":7,"package":1}],"package":0
; MULTI: "index":0,"name":"shared","originals":[{"index":7,"package":0},{"index":9,"package":1}],"package":2
target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
@shared = global [2 x i64] zeroinitializer, section ".bss", !goobj.symbol.index !0
!0 = !{i32 7, i16 0}

!goobj.config = !{!900}
!900 = !{!"declaration-id", !"indexed-bss", !"0", !"0", !"0100000000000000"}
!901 = !{}

!llvm.module.flags = !{!902}
!902 = !{i32 1, !"goobj.abi", !903}
!903 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !901}
