; Requesting a stronger clone must not legalize an unguarded lower operation.
; CHECK: error: GoALLC CPU requirement x86.avx2 survives in function bad<goallc.fmv.baseline> without the required target features

target triple = "x86_64-unknown-linux-goobj"
@runtime.goallcCPUFeatures = external global i64

define goabiinternal <32 x i8> @bad(<32 x i8> %x, <32 x i8> %y) #0 {
  %sum = add <32 x i8> %x, %y, !goallc.cpu.requires !1
  ret <32 x i8> %sum
}
attributes #0 = { "goallc.cpu.feature-floor"="x86.avx" "goallc.cpu.multiversion"="x86.avx512" }
!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}
!1 = !{!"x86.avx2"}
