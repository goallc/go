target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-n8:16:32:64-S128"
; CHECK: @leaf.counter = external {{.*}}global i64, !goobj.import
; CHECK: @leaf.identity = external {{.*}}constant i8, !goobj.import
; CHECK-LABEL: define goabiinternal {{.*}}ptr @callee.Identity()
; CHECK: ret ptr @leaf.identity
; CHECK-LABEL: define goabiinternal i64 @callee.Read()
; CHECK: load volatile i64, ptr @leaf.counter
; CHECK: !goobj.imports = !{![[LEAF:[0-9]+]]}
; CHECK: ![[LEAF]] = !{!"leaf", !"leaf", !"0300000000000000"}

define goabiinternal ptr @callee.Identity() !goobj.symbol.index !0 {
  %p = call goabiinternal ptr @leaf.Identity()
  ret ptr %p
}
define goabiinternal i64 @callee.Read() !goobj.symbol.index !1 {
  %n = call goabiinternal i64 @leaf.Read()
  ret i64 %n
}
declare !goobj.import !2 goabiinternal ptr @leaf.Identity()
declare !goobj.import !3 goabiinternal i64 @leaf.Read()
!goobj.imports = !{!4}
!0 = !{i32 0}
!1 = !{i32 1}
!2 = !{!"leaf", i32 0, i8 0}
!3 = !{!"leaf", i32 1, i8 0}
!4 = !{!"leaf", !"leaf", !"0300000000000000"}

!goobj.config = !{!900}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"callee-id", !"callee", !"0", !"0", !"0", !901, !"0200000000000000"}
!901 = !{}
