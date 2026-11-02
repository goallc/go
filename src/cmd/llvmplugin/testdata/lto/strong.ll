target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
@choice = global i64 6, !goobj.symbol.nonpackage !0, !goobj.gotype !1, !goobj.keep !1, !goobj.marker_relocs !2
!0 = !{i1 true}
!goobj.config = !{!900}
!goobj.imports = !{}
!900 = !{!"strong-id", !"strong", !"0", !"0", !"0100000000000000"}
!901 = !{}

!llvm.module.flags = !{!902}
!902 = !{i32 1, !"goobj.abi", !903}
!903 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !901}

@type.strong = external global i8
@llvm.compiler.used = appending global [1 x ptr] [ptr @type.strong], section "llvm.metadata"
!1 = !{ptr @type.strong}
!2 = !{!3}
!3 = !{ptr @type.strong, i32 23, i64 0}
