target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"

@leaf.counter = internal global i64 41, !goobj.keep !3, !goobj.gotype !3
@leaf.identity = internal constant i8 7
@leaf.unreferenced = internal global i64 3, !goobj.symbol.index !2
define goabiinternal ptr @leaf.Identity() !goobj.symbol.index !0 {
  ret ptr @leaf.identity
}
define goabiinternal i64 @leaf.Read() !goobj.symbol.index !1 {
  %n = load volatile i64, ptr @leaf.counter
  ret i64 %n
}
!0 = !{i32 0}
!1 = !{i32 1}
!2 = !{i32 8}
!3 = !{ptr @leaf.identity}

!goobj.config = !{!900}
!900 = !{!"leaf-id", !"leaf", !"0", !"0", !"0300000000000000"}
!901 = !{}

!llvm.module.flags = !{!902}
!902 = !{i32 1, !"goobj.abi", !903}
!903 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !901}
