; DUPLICATE: goallc-lto: duplicate package prefix: caller
; CHECK-LABEL: define goabiinternal {{.*}}i64 @caller.Answer()
; CHECK-NOT: call
; CHECK: ret i64 42
; CHECK-LABEL: define goabiinternal {{.*}}i64 @caller.Opaque
; CHECK: call goabiinternal i64 @callee.Opaque
; CHECK-LABEL: define goabiinternal {{.*}}i64 @caller.Restricted
; CHECK: call goabiinternal i64 @callee.Restricted
; CHECK: declare !goobj.import ![[IMPORT:[0-9]+]] goabiinternal i64 @callee.Opaque
; CHECK: !goobj.config = !{![[CONFIG:[0-9]+]]}
; CHECK: ![[CONFIG]] = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"caller-id", !"caller"
; CHECK: ![[IMPORT]] = !{!"callee", i32 4, i8 0}
target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-n8:16:32:64-S128"
@llvm.compiler.used = appending global [3 x ptr] [ptr @caller.Answer, ptr @caller.Opaque, ptr @caller.Restricted], section "llvm.metadata"
define goabiinternal i64 @caller.Answer() !goobj.symbol.index !0 {
  %n = call goabiinternal i64 @callee.Add(i64 35)
  ret i64 %n
}
define goabiinternal i64 @caller.Opaque(i64 %x) !goobj.symbol.index !5 {
  %n = call goabiinternal i64 @callee.Opaque(i64 %x)
  ret i64 %n
}
declare !goobj.import !3 goabiinternal i64 @callee.Add(i64)
declare !goobj.import !6 goabiinternal i64 @callee.Opaque(i64)
define goabiinternal i64 @caller.Restricted(i64 %x) !goobj.symbol.index !7 {
  %n = call goabiinternal i64 @callee.Restricted(i64 %x)
  ret i64 %n
}
declare !goobj.import !8 goabiinternal i64 @callee.Restricted(i64)
!goobj.config = !{!1}
!goobj.imports = !{!2}
!0 = !{i32 7}
!1 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"caller-id", !"caller", !"0", !"0", !"0", !4, !"0100000000000000"}
!2 = !{!"callee", !"callee", !"0200000000000000"}
!3 = !{!"callee", i32 3, i8 0}
!4 = !{}
!5 = !{i32 8}
!6 = !{!"callee", i32 4, i8 0}
!7 = !{i32 9}
!8 = !{!"callee", i32 5, i8 0}
