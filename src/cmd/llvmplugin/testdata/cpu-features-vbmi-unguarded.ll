; A GOAMD64=v4 baseline / AVX512 ABI floor does not supply VBMI.
; CHECK: error: GoALLC CPU requirement x86.avx512vbmi survives in function bad<goallc.fmv.baseline> without the required target features
target triple = "x86_64-unknown-linux-goobj"
@runtime.goallcCPUFeatures = external global i64
define goabiinternal <64 x i8> @bad(<64 x i8> %x, <64 x i8> %y) #0 {
  %result = add <64 x i8> %x, %y, !goallc.cpu.requires !1
  ret <64 x i8> %result
}
attributes #0 = { "goallc.cpu.feature-floor"="x86.avx512" "goallc.cpu.multiversion"="x86.avx512vbmi" }
!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v4"}
!1 = !{!"x86.avx512vbmi"}
