; OBJ: "name": "shared"
; OBJ: "kind": "SNOPTRBSS"
; OBJ: "size": 32
; OBJ-NEXT: "align": 32
; OBJ: "type": "gotype"
; OBJ: "sym_index": 1
; OBJ: "references": [
; OBJ: "name": "type.large"
target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
; Size, section and type come from this input; LLVM supplies max alignment.
@shared = global [4 x i64] zeroinitializer, section ".noptrbss", align 8, !goobj.symbol.nonpackage !0, !goobj.gotype !1
@type.large = external global i8
!0 = !{i1 true}
!1 = !{ptr @type.large}
!goobj.config = !{!900}
!goobj.imports = !{}
!900 = !{!"bss-large-id", !"bss-large", !"0", !"0", !"0100000000000000"}
!901 = !{}

!llvm.module.flags = !{!902}
!902 = !{i32 1, !"goobj.abi", !903}
!903 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !901}
@llvm.compiler.used = appending global [1 x ptr] [ptr @type.large], section "llvm.metadata"
