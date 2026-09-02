; A Midway-style AVX floor does not make an unguarded AVX2 operation legal.
; CHECK: error: GoALLC CPU requirement x86.avx2 survives in function bad<goallc.fmv.baseline> without the required target features

target triple = "x86_64-unknown-linux-gnu"

@runtime.goallcCPUFeatures = external global i8
declare double @llvm.floor.f64(double)

define double @bad(double %x) #0 {
entry:
  %rounded = call double @llvm.floor.f64(double %x), !goallc.cpu.requires !1
  ret double %rounded
}

attributes #0 = { "goallc.cpu.feature-floor"="x86.avx" "goallc.cpu.multiversion"="x86.avx2" "target-cpu"="x86-64" }

!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}
!1 = !{!"x86.avx2"}
