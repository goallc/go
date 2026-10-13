target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
@local = internal global i64 1, !goobj.symbol.index !0
; CHECK: @local = {{.*}}global i64 1, {{.*}}!goobj.symbol.index
; CHECK: @local.goallc.lto.callee = external {{.*}}global i64, !goobj.import ![[IMPORT:[0-9]+]]
; CHECK-LABEL: define goabiinternal i64 @caller.Read()
; CHECK-NOT: call
; CHECK: load volatile i64, ptr @local.goallc.lto.callee
; CHECK: load volatile i64, ptr @local
; CHECK: ![[IMPORT]] = !{!"callee", i32 0, i8 0}
define goabiinternal i64 @caller.Read() !goobj.symbol.index !1 {
 %a = call goabiinternal i64 @callee.Read()
 %b = load volatile i64, ptr @local
 %c = add i64 %a, %b
 ret i64 %c
}
declare !goobj.import !2 goabiinternal i64 @callee.Read()
!2 = !{!"callee", i32 1, i8 0}
!goobj.imports = !{!3}
!3 = !{!"callee", !"callee", !"0100000000000000"}
!0 = !{i32 0, i16 -1}
!1 = !{i32 1}
!goobj.config = !{!900}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"id", !"caller", !"0", !"0", !"0", !901, !"0100000000000000"}
!901 = !{}
