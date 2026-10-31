target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
!goobj.config = !{!0}
!goobj.imports = !{}
!0 = !{!"id", !"dep", !"0", !"0", !"0100000000000000"}
!1 = !{i1 true}
!2 = !{i32 0}
!3 = !{}
!llvm.module.flags = !{!4}
!4 = !{i32 1, !"goobj.abi", !5}
!5 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !3}
@dep.closure = global ptr @dep.Closure, !goobj.symbol.index !2
define goabiinternal i64 @dep.Closure() noinline !goobj.content_addressable !1 {
  ret i64 11
}
