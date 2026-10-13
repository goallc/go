; CHECK-LABEL: define goabiinternal {{.*}}i64 @callee.Add(i64
; CHECK-SAME: !goobj.symbol.index ![[INDEX:[0-9]+]]
; CHECK: ![[INDEX]] = !{i32 3}
target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-n8:16:32:64-S128"
@llvm.compiler.used = appending global [3 x ptr] [ptr @callee.Add, ptr @callee.Opaque, ptr @callee.Restricted], section "llvm.metadata"
define goabiinternal i64 @callee.Add(i64 %x) !goobj.symbol.index !0 {
  %n = add i64 %x, 7
  ret i64 %n
}
define goabiinternal i64 @callee.Opaque(i64 %x) noinline !goobj.symbol.index !3 {
  %n = add i64 %x, 9
  ret i64 %n
}
define goabiinternal i64 @callee.Restricted(i64 %x) !goobj.symbol.index !4 {
  %n = call goabiinternal i64 @"runtime.private<linkname>"(i64 %x)
  ret i64 %n
}
declare goabiinternal i64 @"runtime.private<linkname>"(i64)
!goobj.config = !{!1}
!0 = !{i32 3}
!1 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"callee-id", !"callee", !"0", !"0", !"0", !2, !"0200000000000000"}
!2 = !{}
!3 = !{i32 4}
!4 = !{i32 5}
