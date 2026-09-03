target triple = "x86_64-unknown-linux-goobj"

@runtime.goallcCPUFeatures = external global i64
@runtime.x86HasAVX2 = external global i8

define goabiinternal <32 x i8> @wide(<32 x i8> %x, <32 x i8> %y) #0 {
entry:
  %flag = load i8, ptr @runtime.x86HasAVX2, align 1, !goallc.cpu.guard !1
  %enabled = icmp ne i8 %flag, 0
  br i1 %enabled, label %feature, label %fallback

feature:
  %sum = add <32 x i8> %x, %y, !goallc.cpu.requires !1
  ret <32 x i8> %sum

fallback:
  ret <32 x i8> %x
}

attributes #0 = { "goallc.cpu.feature-floor"="x86.avx" "goallc.cpu.multiversion"="x86.avx2" "target-cpu"="x86-64" }

!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}
!1 = !{!"x86.avx2"}

; A wide-vector Go ABI function transfers its YMM argument/result carrier
; through the dispatcher and resolver, so all three layers retain the AVX
; floor. The instruction-specialized implementation additionally gets AVX2.
; CHECK-LABEL: define goabiinternal <32 x i8> @wide(
; CHECK-SAME: #[[DISPATCH:[0-9]+]]
; CHECK: tail call goabiinternal <32 x i8> %target

; CHECK-LABEL: define internal goabiinternal <32 x i8> @"wide<goallc.fmv.baseline>"(
; CHECK-SAME: #[[BASELINE:[0-9]+]]
; CHECK-NOT: add <32 x i8>
; CHECK: ret <32 x i8> %x

; CHECK-LABEL: define internal goabiinternal <32 x i8> @"wide<goallc.fmv.avx2>"(
; CHECK-SAME: #[[AVX2:[0-9]+]]
; CHECK: add <32 x i8> %x, %y

; CHECK-LABEL: define internal goabiinternal <32 x i8> @"wide<goallc.fmv.resolve>"(
; CHECK-SAME: #[[RESOLVER:[0-9]+]]
; CHECK: musttail call goabiinternal <32 x i8>

; CHECK-DAG: attributes #[[DISPATCH]] = { "go-nosplit" "goallc.cpu.tail-transfers" "target-cpu"="x86-64" "target-features"="+avx" }
; CHECK-DAG: attributes #[[BASELINE]] = { "target-cpu"="x86-64" "target-features"="+avx" }
; CHECK-DAG: attributes #[[AVX2]] = { "target-cpu"="x86-64" "target-features"="+avx,+avx2" }
; CHECK-DAG: attributes #[[RESOLVER]] = { noinline "go-nosplit" "target-cpu"="x86-64" "target-features"="+avx" }
