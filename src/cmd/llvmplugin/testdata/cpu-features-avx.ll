; AVX is both a valid function-entry floor and a normal FMV profile. Keep the
; dispatch form covered so the common profile registry cannot silently accept
; x86.avx without generating its implementation clone.

target triple = "x86_64-unknown-linux-gnu"

@runtime.goallcCPUFeatures = external global i8

declare <16 x i8> @fallback(<16 x i8>, <16 x i8>)

; CHECK-LABEL: define <16 x i8> @add(
; CHECK: load atomic ptr, ptr @add.goallc.fmv.slot monotonic
; CHECK: tail call <16 x i8> %target
define <16 x i8> @add(<16 x i8> %x, <16 x i8> %y) #0 {
entry:
  %flag = load i8, ptr @runtime.goallcCPUFeatures, align 1, !goallc.cpu.guard !1
  %enabled = icmp ne i8 %flag, 0
  br i1 %enabled, label %feature, label %soft

feature:
  %sum = add <16 x i8> %x, %y, !goallc.cpu.requires !1
  br label %done

soft:
  %fallback = call <16 x i8> @fallback(<16 x i8> %x, <16 x i8> %y)
  br label %done

done:
  %result = phi <16 x i8> [ %sum, %feature ], [ %fallback, %soft ]
  ret <16 x i8> %result
}

; CHECK-LABEL: define internal <16 x i8> @"add<goallc.fmv.baseline>"(
; CHECK-NOT: add <16 x i8>
; CHECK: call <16 x i8> @fallback

; CHECK-LABEL: define internal <16 x i8> @"add<goallc.fmv.avx>"(
; CHECK-SAME: #[[AVX:[0-9]+]]
; CHECK: add <16 x i8>
; CHECK-NOT: call <16 x i8> @fallback

; CHECK-LABEL: define internal <16 x i8> @"add<goallc.fmv.resolve>"(
; CHECK: and i64 %features, 64
; CHECK: and i64 %features, 16
; CHECK: select i1 {{.*}}, ptr @"add<goallc.fmv.avx>", ptr @"add<goallc.fmv.baseline>"
; CHECK: attributes #[[AVX]] = {{.*}}"target-features"="+avx"

attributes #0 = { "goallc.cpu.multiversion"="x86.avx" "target-cpu"="x86-64" }

!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}
!1 = !{!"x86.avx"}
