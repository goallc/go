target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
; Alignment comes from this input even when the larger definition wins.
@shared = global i64 zeroinitializer, section ".bss", align 32, !goobj.symbol.nonpackage !0, !goobj.gotype !1
@type.small = external global i8
!0 = !{i1 true}
!1 = !{ptr @type.small}
!goobj.config = !{!900}
!goobj.imports = !{}
!900 = !{!"bss-small-id", !"bss-small", !"0", !"0", !"0100000000000000"}
!901 = !{}

!llvm.module.flags = !{!902}
!902 = !{i32 1, !"goobj.abi", !903}
!903 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !901}
@llvm.compiler.used = appending global [1 x ptr] [ptr @type.small], section "llvm.metadata"
