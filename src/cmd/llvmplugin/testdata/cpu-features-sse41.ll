; RUN: opt -load-pass-plugin=%plugin -passes=goallc-cpu-features,goallc-cpu-features -S %s | FileCheck %s
;
; The early pass clones the guarded feature path, specializes each guard, and
; leaves one public lazy dispatcher. Running it twice is deliberately part of
; the test: the module marker must make every entry idempotent.

target triple = "x86_64-unknown-linux-gnu"

@runtime.goallcCPUFeatures = external global i8

; CHECK: @round.goallc.fmv.slot = internal global ptr null

declare double @fallback(double)
declare double @llvm.floor.f64(double)

; CHECK-LABEL: define double @round(
; CHECK: entry:
; CHECK: load atomic ptr, ptr @round.goallc.fmv.slot acquire
; CHECK: br i1 {{.*}}, label %dispatch, label %resolve
; CHECK: dispatch:
; CHECK: musttail call double %target(double %x)
; CHECK-NEXT: ret double
; CHECK: resolve:
; CHECK: load atomic i64, ptr @runtime.goallcCPUFeatures acquire
; CHECK: and i64 %features, 64
; CHECK: br i1
; CHECK: uninitialized:
; CHECK: musttail call double @round.goallc.fmv.baseline(double %x)
; CHECK: select:
; CHECK: and i64 %features, 4
; CHECK: select i1 {{.*}}, ptr @round.goallc.fmv.sse41, ptr @round.goallc.fmv.baseline
; CHECK: store atomic ptr {{.*}}, ptr @round.goallc.fmv.slot release
; CHECK: musttail call double {{.*}}(double %x)
define double @round(double %x) #0 {
entry:
  %flag = load i8, ptr @runtime.goallcCPUFeatures, align 1, !goallc.cpu.guard !1
  %enabled = icmp ne i8 %flag, 0
  br i1 %enabled, label %feature, label %fallback

feature:
  %rounded = call double @llvm.floor.f64(double %x), !goallc.cpu.requires !1
  br label %done

fallback:
  %soft = call double @fallback(double %x)
  br label %done

done:
  %result = phi double [ %rounded, %feature ], [ %soft, %fallback ]
  ret double %result
}

; CHECK-LABEL: define internal double @round.goallc.fmv.baseline(
; CHECK-NOT: llvm.floor
; CHECK: call double @fallback(double %x)
; CHECK: ret double

; CHECK-LABEL: define internal double @round.goallc.fmv.sse41(
; CHECK-SAME: #[[SSE41:[0-9]+]]
; CHECK: call double @llvm.floor.f64(double %x), !goallc.cpu.requires
; CHECK-NOT: call double @fallback
; CHECK: ret double

; CHECK: attributes #[[SSE41]] = {{.*}}"target-cpu"="x86-64" {{.*}}"target-features"="+sse4.1"
; CHECK: !goallc.cpu.fmv.done = !{![[DONE:[0-9]+]]}
; CHECK: ![[DONE]] = !{!"goallc.cpu.v1"}

attributes #0 = { "goallc.cpu.multiversion"="x86.sse41" "target-cpu"="x86-64" }

!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}
!1 = !{!"x86.sse41"}
