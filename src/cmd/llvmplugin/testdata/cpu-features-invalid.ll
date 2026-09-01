; CHECK: error: GoALLC CPU requirement x86.sse41 survives in function bad<goallc.fmv.baseline> without the required target features

target triple = "x86_64-unknown-linux-gnu"

@runtime.goallcCPUFeatures = external global i8
declare double @llvm.floor.f64(double)

define double @bad(double %x) #0 {
entry:
  %rounded = call double @llvm.floor.f64(double %x), !goallc.cpu.requires !1
  ret double %rounded
}

attributes #0 = { "goallc.cpu.multiversion"="x86.sse41" "target-cpu"="x86-64" }

!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}
!1 = !{!"x86.sse41"}
