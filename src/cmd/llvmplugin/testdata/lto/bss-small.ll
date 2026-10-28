target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
; Alignment comes from this input even when the larger definition wins.
@shared = global i64 zeroinitializer, section ".bss", align 32, !goobj.symbol.nonpackage !0
@type.small = external global i8
!goobj.gotype = !{!1}
!0 = !{i1 true}
!1 = !{ptr @shared, ptr @type.small}
!goobj.config = !{!900}
!goobj.imports = !{}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"bss-small-id", !"bss-small", !"0", !"0", !"0", !901, !"0100000000000000"}
!901 = !{}
