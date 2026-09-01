target triple = "x86_64-unknown-linux-gnu"

@runtime.goallcCPUFeatures = external global i8

declare double @fallback(double, double, double)
declare double @llvm.fma.f64(double, double, double)

; CHECK: @fma.goallc.fmv.slot = internal global ptr null
; CHECK-LABEL: define double @fma(
; CHECK: musttail call double %target(double %x, double %y, double %z)
; CHECK: resolve:
; CHECK: and i64 %features, 64
; CHECK: and i64 %features, 32
; CHECK: icmp eq i64 {{.*}}, 32
; CHECK: select i1 {{.*}}, ptr @fma.goallc.fmv.fma, ptr @fma.goallc.fmv.baseline

define double @fma(double %x, double %y, double %z) #0 {
entry:
  %flag = load i8, ptr @runtime.goallcCPUFeatures, align 1, !goallc.cpu.guard !1
  %enabled = icmp ne i8 %flag, 0
  br i1 %enabled, label %feature, label %fallback

feature:
  %value = call double @llvm.fma.f64(double %x, double %y, double %z), !goallc.cpu.requires !1
  br label %done

fallback:
  %soft = call double @fallback(double %x, double %y, double %z)
  br label %done

done:
  %result = phi double [ %value, %feature ], [ %soft, %fallback ]
  ret double %result
}

; CHECK-LABEL: define internal double @fma.goallc.fmv.baseline(
; CHECK-NOT: llvm.fma
; CHECK: call double @fallback

; CHECK-LABEL: define internal double @fma.goallc.fmv.fma(
; CHECK-SAME: #[[FMA:[0-9]+]]
; CHECK: call double @llvm.fma.f64
; CHECK-NOT: call double @fallback

; CHECK: attributes #[[FMA]] = {{.*}}"target-cpu"="x86-64-v2" {{.*}}"target-features"="+fma"

attributes #0 = { "goallc.cpu.multiversion"="x86.fma" "target-cpu"="x86-64-v2" }

!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v2"}
!1 = !{!"x86.fma"}
