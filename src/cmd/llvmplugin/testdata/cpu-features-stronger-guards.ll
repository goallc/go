; A stronger effective guard supplies lower instruction capabilities.
; The dispatch condition must remain the stronger effective boolean.

target triple = "x86_64-unknown-linux-goobj"
@runtime.goallcCPUFeatures = external global i64
@runtime.x86HasAVX512 = external global i8
define goabiinternal <32 x i8> @wide(<32 x i8> %x, <32 x i8> %y) #0 {
entry:
  %flag = load i8, ptr @runtime.x86HasAVX512, align 1, !goallc.cpu.guard !1
  %enabled = icmp ne i8 %flag, 0
  br i1 %enabled, label %feature, label %fallback
feature:
  %sum = add <32 x i8> %x, %y, !goallc.cpu.requires !2
  ret <32 x i8> %sum
fallback:
  ret <32 x i8> %x
}
attributes #0 = { "target-features"="+avx" "goallc.cpu.multiversion"="x86.avx512" "target-cpu"="x86-64" }
!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}
!1 = !{!"x86.avx512"}
!2 = !{!"x86.avx2"}

; CHECK-LABEL: define internal goabiinternal <32 x i8> @"wide<goallc.fmv.baseline>"
; CHECK-NOT: add <32 x i8>
; CHECK: ret <32 x i8> %x
; CHECK-LABEL: define internal goabiinternal <32 x i8> @"wide<goallc.fmv.avx512>"
; CHECK: add <32 x i8> %x, %y
; CHECK-LABEL: define internal goabiinternal <32 x i8> @"wide<goallc.fmv.resolve>"
; CHECK: and i64 %features, 1024

; Both booleans are requested. The AVX512-only clone must take the false
; AVX2 branch while still accepting its AVX2 instruction.
@runtime.x86HasAVX2 = external global i8

define goabiinternal <32 x i8> @mixed(<32 x i8> %x, <32 x i8> %y) #1 {
entry:
  %high = load i8, ptr @runtime.x86HasAVX512, !goallc.cpu.guard !1
  %high.enabled = icmp ne i8 %high, 0
  br i1 %high.enabled, label %high.path, label %low.path
high.path:
  %low = load i8, ptr @runtime.x86HasAVX2, !goallc.cpu.guard !2
  %low.enabled = icmp ne i8 %low, 0
  br i1 %low.enabled, label %both, label %only.high
both:
  %sum = add <32 x i8> %x, %y, !goallc.cpu.requires !2
  ret <32 x i8> %sum
only.high:
  %twice = add <32 x i8> %x, %x, !goallc.cpu.requires !2
  ret <32 x i8> %twice
low.path:
  %other = load i8, ptr @runtime.x86HasAVX2, !goallc.cpu.guard !2
  %other.enabled = icmp ne i8 %other, 0
  br i1 %other.enabled, label %only.low, label %fallback
only.low:
  %low.sum = add <32 x i8> %y, %y, !goallc.cpu.requires !2
  ret <32 x i8> %low.sum
fallback:
  ret <32 x i8> %x
}
attributes #1 = { "target-features"="+avx" "goallc.cpu.multiversion"="x86.avx2,x86.avx512" }

; CHECK-LABEL: define internal goabiinternal <32 x i8> @"mixed<goallc.fmv.baseline>"
; CHECK-NOT: add <32 x i8>
; CHECK: ret <32 x i8> %x
; CHECK-LABEL: define internal goabiinternal <32 x i8> @"mixed<goallc.fmv.avx2>"
; CHECK: add <32 x i8> %y, %y
; CHECK-LABEL: define internal goabiinternal <32 x i8> @"mixed<goallc.fmv.avx512>"
; CHECK: add <32 x i8> %x, %x
; CHECK-LABEL: define internal goabiinternal <32 x i8> @"mixed<goallc.fmv.avx2-avx512>"
; CHECK: add <32 x i8> %x, %y
; CHECK-LABEL: define internal goabiinternal <32 x i8> @"mixed<goallc.fmv.resolve>"
; CHECK: and i64 %features, 512
; CHECK: and i64 %features, 1024
; CHECK: and i64 %features, 1536

; The ABI floor supplies instructions, not an effective Go boolean.
define i8 @floor_boolean() #2 {
  %low = load i8, ptr @runtime.x86HasAVX2, !goallc.cpu.guard !2
  ret i8 %low
}
attributes #2 = { "target-features"="+avx,+avx2" "goallc.cpu.multiversion"="x86.avx2" }
; CHECK-LABEL: define internal i8 @"floor_boolean<goallc.fmv.baseline>"
; CHECK: ret i8 0
; CHECK-LABEL: define internal i8 @"floor_boolean<goallc.fmv.avx2>"
; CHECK: ret i8 1
; CHECK-LABEL: define internal i8 @"floor_boolean<goallc.fmv.resolve>"
; CHECK: and i64 %features, 512
