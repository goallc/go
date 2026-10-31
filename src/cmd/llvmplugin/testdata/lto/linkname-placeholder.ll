target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
; A Go linkname BSS placeholder may denote a function.
@target = global i64 zeroinitializer, section ".bss", !goobj.symbol.nonpackage !0, !goobj.symbol.flags !1
!0 = !{i1 true}
!1 = !{i8 0, i8 16}
!goobj.config = !{!900}
!goobj.imports = !{}
!900 = !{!"linkname-placeholder-id", !"linkname-placeholder", !"0", !"0", !"0100000000000000"}
!901 = !{}

!llvm.module.flags = !{!902}
!902 = !{i32 1, !"goobj.abi", !903}
!903 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !901}
