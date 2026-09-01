; SSE4.1 and FMA are independently controllable Go features. A mixed function
; needs individual and combination variants rather than an implication from
; FMA to SSE4.1.

target triple = "x86_64-unknown-linux-gnu"

@runtime.goallcCPUFeatures = external global i8

declare double @fallback.round(double)
declare double @fallback.fma(double, double, double)
declare double @llvm.floor.f64(double)
declare double @llvm.fma.f64(double, double, double)

; CHECK-LABEL: define double @mixed(
; CHECK: load atomic ptr, ptr @mixed.goallc.fmv.slot monotonic
; CHECK: tail call double %target

; CHECK-LABEL: define internal double @mixed.goallc.fmv.baseline(
; CHECK-NOT: llvm.floor
; CHECK-NOT: llvm.fma
; CHECK: call double @fallback.round
; CHECK: call double @fallback.fma

; CHECK-LABEL: define internal double @mixed.goallc.fmv.sse41(
; CHECK: call double @llvm.floor.f64
; CHECK-NOT: llvm.fma
; CHECK: call double @fallback.fma

; CHECK-LABEL: define internal double @mixed.goallc.fmv.fma(
; CHECK-NOT: llvm.floor
; CHECK: call double @fallback.round
; CHECK: call double @llvm.fma.f64

; CHECK-LABEL: define internal double @mixed.goallc.fmv.sse41-fma(
; CHECK: call double @llvm.floor.f64
; CHECK: call double @llvm.fma.f64

; CHECK-LABEL: define internal double @mixed.goallc.fmv.resolve(
; CHECK: load i64, ptr @runtime.goallcCPUFeatures
; CHECK: and i64 %features, 4
; CHECK: select i1 {{.*}}, ptr @mixed.goallc.fmv.sse41, ptr @mixed.goallc.fmv.baseline
; CHECK: and i64 %features, 32
; CHECK: select i1 {{.*}}, ptr @mixed.goallc.fmv.fma
; CHECK: and i64 %features, 36
; CHECK: select i1 {{.*}}, ptr @mixed.goallc.fmv.sse41-fma

define double @mixed(double %x, double %y, double %z) #0 {
entry:
  %sse.flag = load i8, ptr @runtime.goallcCPUFeatures, align 1, !goallc.cpu.guard !1
  %sse.enabled = icmp ne i8 %sse.flag, 0
  br i1 %sse.enabled, label %sse, label %soft.round

sse:
  %rounded = call double @llvm.floor.f64(double %x), !goallc.cpu.requires !1
  br label %after.round

soft.round:
  %soft.rounded = call double @fallback.round(double %x)
  br label %after.round

after.round:
  %round.result = phi double [ %rounded, %sse ], [ %soft.rounded, %soft.round ]
  %fma.flag = load i8, ptr @runtime.goallcCPUFeatures, align 1, !goallc.cpu.guard !2
  %fma.enabled = icmp ne i8 %fma.flag, 0
  br i1 %fma.enabled, label %fma, label %soft.fma

fma:
  %fused = call double @llvm.fma.f64(double %round.result, double %y, double %z), !goallc.cpu.requires !2
  br label %done

soft.fma:
  %soft.fused = call double @fallback.fma(double %round.result, double %y, double %z)
  br label %done

done:
  %result = phi double [ %fused, %fma ], [ %soft.fused, %soft.fma ]
  ret double %result
}

attributes #0 = { "goallc.cpu.multiversion"="x86.fma,x86.sse41" "target-cpu"="x86-64" }

!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}
!1 = !{!"x86.sse41"}
!2 = !{!"x86.fma"}
