target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-n8:16:32:64-S128"
; CHECK: @leaf.counter = {{.*}}global i64 41, !goobj.symbol.index ![[COUNTER:[0-9]+]]
; CHECK: @leaf.identity = {{.*}}constant i8 7, !goobj.symbol.index ![[IDENTITY:[0-9]+]]
; CHECK: @leaf.unreferenced = {{.*}}global i64 3, !goobj.symbol.index ![[RETAINED:[0-9]+]]
; CHECK: !goobj.keep = !{![[KEEP:[0-9]+]]}
; CHECK: !goobj.gotype = !{![[KEEP]]}
; CHECK-DAG: ![[KEEP]] = !{ptr @leaf.counter, ptr @leaf.identity}
; CHECK-DAG: ![[COUNTER]] = !{i32 9, i16 -1}
; CHECK-DAG: ![[IDENTITY]] = !{i32 10, i16 -1}
; CHECK-DAG: ![[RETAINED]] = !{i32 8}
; CHECK-NOT: goallc.lto.owner

@leaf.counter = internal global i64 41
@leaf.identity = internal constant i8 7
@leaf.unreferenced = internal global i64 3, !goobj.symbol.index !2
define goabiinternal ptr @leaf.Identity() !goobj.symbol.index !0 {
  ret ptr @leaf.identity
}
define goabiinternal i64 @leaf.Read() !goobj.symbol.index !1 {
  %n = load volatile i64, ptr @leaf.counter
  ret i64 %n
}
!goobj.keep = !{!3}
!goobj.gotype = !{!3}
!0 = !{i32 0}
!1 = !{i32 1}
!2 = !{i32 8}
!3 = !{ptr @leaf.counter, ptr @leaf.identity}

!goobj.config = !{!900}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"leaf-id", !"leaf", !"0", !"0", !"0", !901, !"0300000000000000"}
!901 = !{}
