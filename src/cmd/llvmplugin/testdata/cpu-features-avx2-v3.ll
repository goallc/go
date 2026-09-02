; GOAMD64=v3 makes AVX2 part of the module baseline. An AVX2 requirement must
; therefore be legal even in a clone selected for an unrelated profile.

target triple = "x86_64-unknown-linux-gnu"

@runtime.goallcCPUFeatures = external global i64

define <32 x i8> @v3_vec_add(<32 x i8> %x, <32 x i8> %y) #0 {
entry:
  %sum = add <32 x i8> %x, %y, !goallc.cpu.requires !1
  ret <32 x i8> %sum
}

attributes #0 = { "goallc.cpu.multiversion"="x86.sse41" "target-cpu"="x86-64-v3" }

!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v3"}
!1 = !{!"x86.avx2"}

; CHECK-LABEL: define internal <32 x i8> @"v3_vec_add<goallc.fmv.baseline>"
; CHECK: add <32 x i8>
; CHECK-LABEL: define internal <32 x i8> @"v3_vec_add<goallc.fmv.sse41>"
; CHECK: add <32 x i8>
