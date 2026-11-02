; CHECK: unknown directive
; CHECK-NEXT: .symver dep.f,dep.f@VERS_1
; GoObj rejects ELF symbol-version directives before the ThinLTO importer can
; selectively copy them. Ordinary module assembly needs no save/restore layer.
target triple = "x86_64-unknown-linux-goobj"
module asm ".symver dep.f,dep.f@VERS_1"
define goabiinternal void @dep.f() !goobj.symbol.index !0 {
  ret void
}
!0 = !{i32 0, i16 1}
!goobj.config = !{!1}
!1 = !{!"dep-id", !"dep", !"0", !"0", !"0100000000000000"}
!llvm.module.flags = !{!2}
!2 = !{i32 1, !"goobj.abi", !3}
!3 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !4}
!4 = !{}
