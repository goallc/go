target triple = "x86_64-unknown-linux-gnu"

@runtime.goallcCPUFeatures = external global i64
@runtime.x86HasAVX512VBMI = external global i8
@runtime.x86HasAVX2 = external global i8

; The VBMI predicate grants lower instruction capabilities without folding
; independently disableable AVX2 source tests to true.
define internal i64 @vbmi(i64 %x) #0 {
  %flag = load i8, ptr @runtime.x86HasAVX512VBMI, !goallc.cpu.guard !1
  %enabled = icmp ne i8 %flag, 0
  br i1 %enabled, label %feature, label %fallback
feature:
  %avx2 = load i8, ptr @runtime.x86HasAVX2, !goallc.cpu.guard !2
  %test = zext i8 %avx2 to i64
  %lower = add i64 %x, %test, !goallc.cpu.requires !3
  %value = add i64 %lower, 1, !goallc.cpu.requires !1
  ret i64 %value
fallback:
  ret i64 %x
}

; CHECK-LABEL: define internal i64 @"vbmi<goallc.fmv.baseline>"(
; CHECK-NOT: add i64
; CHECK: ret i64 %x
; CHECK-LABEL: define internal i64 @"vbmi<goallc.fmv.avx512vbmi>"(
; CHECK-SAME: #[[VBMI:[0-9]+]]
; CHECK-NOT: load i8
; CHECK-NOT: %lower = add
; CHECK: %value = add i64 %x, 1
; CHECK-LABEL: define internal i64 @"vbmi<goallc.fmv.avx2-avx512vbmi>"(
; CHECK: %lower = add i64 %x, 1
; CHECK: %value = add i64 %lower, 1
; CHECK-LABEL: define internal i64 @"vbmi<goallc.fmv.resolve>"(
; CHECK: and i64 %features, 8192
; CHECK: select i1 {{.*}}, ptr @"vbmi<goallc.fmv.avx512vbmi>", ptr
; CHECK: and i64 %features, 8704
; CHECK: attributes #[[VBMI]] = {{.*}}"target-features"="+avx,+avx2,+avx512f,+avx512cd,+avx512bw,+avx512dq,+avx512vl,+avx512vbmi"

attributes #0 = { "goallc.cpu.multiversion"="x86.avx2,x86.avx512vbmi" "target-cpu"="x86-64" }
!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}
!1 = !{!"x86.avx512vbmi"}
!2 = !{!"x86.avx2"}
!3 = !{!"x86.avx512"}
