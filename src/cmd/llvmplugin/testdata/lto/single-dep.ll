target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
@dep.value = global i64 9, !goobj.symbol.index !4
define goabiinternal i64 @dep.Add(i64 %x) !goobj.symbol.index !1 {
  %v = add i64 %x, 7
  ret i64 %v
}
define goabiinternal i64 @dep.Opaque() noinline !goobj.symbol.index !2 {
  %v = load volatile i64, ptr @dep.value
  ret i64 %v
}
!goobj.config = !{!0}
!goobj.imports = !{}
!0 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"dep-id", !"dep", !"0", !"0", !"0", !3, !"0200000000000000"}
!1 = !{i32 7}
!2 = !{i32 8}
!3 = !{}
!4 = !{i32 9}
