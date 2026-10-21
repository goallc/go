target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
declare goabiinternal void @runtime.external()
define goabiinternal void @dep.Poll() noinline gc "goallc" !goobj.symbol.index !0 {
  call goabiinternal void @runtime.external()
  ret void
}
; Only ordinary assembly could refer to this symbol. LTO must retain it.
define goabiinternal i64 @dep.AssemblyOnly() gc "goallc" !goobj.symbol.index !1 {
  ret i64 19
}
!0 = !{i32 7}
!1 = !{i32 8}
!goobj.config = !{!900}
!goobj.imports = !{}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"codegen-dep-id", !"codegen-dep", !"0", !"0", !"0", !901, !"0100000000000000"}
!901 = !{}
