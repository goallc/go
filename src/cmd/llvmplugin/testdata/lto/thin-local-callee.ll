target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
@local = internal global i64 41, !goobj.symbol.index !0
; CHECK: @local = {{.*}}global i64 41, {{.*}}!goobj.symbol.index
; CHECK-LABEL: define goabiinternal i64 @callee.Read()
; CHECK: load volatile i64, ptr @local
define goabiinternal i64 @callee.Read() !goobj.symbol.index !1 {
 %v = load volatile i64, ptr @local
 ret i64 %v
}
!0 = !{i32 0, i16 -1}
!1 = !{i32 1}
!goobj.config = !{!900}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"id", !"callee", !"0", !"0", !"0", !901, !"0100000000000000"}
!901 = !{}
