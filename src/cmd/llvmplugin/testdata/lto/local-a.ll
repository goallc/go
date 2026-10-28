; LLVM renames colliding locals; both retain their original package/index.
; LOCAL-DAG: @[[A:local[^ ]*]] = dso_local global i64 11, !goobj.symbol.index
; LOCAL-DAG: @[[B:local[^ ]*]] = dso_local global i64 22, !goobj.symbol.index
; LOCAL-DAG: "name":"[[A]]","original_index":7,"package":0
; LOCAL-DAG: "name":"[[B]]","original_index":7,"package":1
target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
@local = internal global i64 11, !goobj.symbol.index !0
!0 = !{i32 7}
!goobj.config = !{!900}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"local-a-id", !"local-a", !"0", !"0", !"0", !901, !"0100000000000000"}
!901 = !{}
