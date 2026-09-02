; GOAMD64=v4 guarantees the same F+CD+BW+DQ+VL set represented by Go's virtual
; HasAVX512 boolean. Treat that profile as part of the module baseline.

target triple = "x86_64-unknown-linux-gnu"

@runtime.goallcCPUFeatures = external global i64

define <64 x i8> @v4_vec_add(<64 x i8> %x, <64 x i8> %y) #0 {
entry:
  %sum = add <64 x i8> %x, %y, !goallc.cpu.requires !1
  ret <64 x i8> %sum
}

attributes #0 = { "goallc.cpu.multiversion"="x86.sse41" "target-cpu"="x86-64-v4" }

!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v4"}
!1 = !{!"x86.avx512"}

; CHECK-LABEL: define internal <64 x i8> @"v4_vec_add<goallc.fmv.baseline>"
; CHECK: add <64 x i8>
; CHECK-LABEL: define internal <64 x i8> @"v4_vec_add<goallc.fmv.sse41>"
; CHECK: add <64 x i8>
