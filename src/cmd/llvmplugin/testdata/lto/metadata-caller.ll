target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
; CHECK: @leaf.counter = external {{.*}}global i64, !goobj.import ![[COUNTER:[0-9]+]]
; CHECK: @leaf.identity = external {{.*}}constant i8, !goobj.import ![[IDENTITY:[0-9]+]]
; CHECK-LABEL: define goabiinternal {{.*}}ptr @caller.Identity()
; CHECK-NOT: call
; CHECK: ret ptr @leaf.identity
; CHECK-LABEL: define goabiinternal i64 @caller.Read()
; CHECK-NOT: call
; CHECK: load volatile i64, ptr @leaf.counter
; CHECK: !goobj.imports = !{![[CALLEE:[0-9]+]], ![[LEAF:[0-9]+]]}
; CHECK-DAG: ![[CALLEE]] = !{!"callee", !"callee", !"0200000000000000"}
; CHECK-DAG: ![[LEAF]] = !{!"leaf", !"leaf", !"0300000000000000"}
; CHECK-DAG: ![[COUNTER]] = !{!"leaf", i32 9, i8 0}
; CHECK-DAG: ![[IDENTITY]] = !{!"leaf", i32 10, i8 0}
; CHECK-NOT: goallc.lto.owner

define goabiinternal ptr @caller.Identity() !goobj.symbol.index !0 {
  %p = call goabiinternal ptr @callee.Identity()
  ret ptr %p
}
define goabiinternal i64 @caller.Read() !goobj.symbol.index !1 {
  %n = call goabiinternal i64 @callee.Read()
  ret i64 %n
}
declare !goobj.import !2 goabiinternal ptr @callee.Identity()
declare !goobj.import !3 goabiinternal i64 @callee.Read()
!goobj.imports = !{!4}
!0 = !{i32 0}
!1 = !{i32 1}
!2 = !{!"callee", i32 0, i8 0}
!3 = !{!"callee", i32 1, i8 0}
!4 = !{!"callee", !"callee", !"0200000000000000"}

!goobj.config = !{!900}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"caller-id", !"caller", !"0", !"0", !"0", !901, !"0100000000000000"}
!901 = !{}
