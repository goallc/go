target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
@shared = global [4 x i64] [i64 1, i64 2, i64 3, i64 4], section ".data", !goobj.symbol.nonpackage !0, !goobj.gotype !1
@type.initialized = external global i8
!0 = !{i1 true}
!1 = !{ptr @type.initialized}
!goobj.config = !{!900}
!goobj.imports = !{}
!900 = !{!"bss-initialized-id", !"bss-initialized", !"0", !"0", !"0100000000000000"}
!901 = !{}

!llvm.module.flags = !{!902}
!902 = !{i32 1, !"goobj.abi", !903}
!903 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !901}
@llvm.compiler.used = appending global [1 x ptr] [ptr @type.initialized], section "llvm.metadata"
