; AVX2 and Go's virtual AVX-512 feature are function-entry floors for Midway's
; 256-bit and 512-bit variants. The pass only adds target features; it must not
; create another dispatcher for an already selected width.

target triple = "x86_64-unknown-linux-goobj"

define goabiinternal <32 x i8> @vec256_add(<32 x i8> %x, <32 x i8> %y) #0 {
entry:
  %sum = add <32 x i8> %x, %y
  ret <32 x i8> %sum
}

define goabiinternal <64 x i8> @vec512_add(<64 x i8> %x, <64 x i8> %y) #1 {
entry:
  %sum = add <64 x i8> %x, %y
  ret <64 x i8> %sum
}

attributes #0 = { "goallc.cpu.feature-floor"="x86.avx2" }
attributes #1 = { "goallc.cpu.feature-floor"="x86.avx512" }

!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}

; CHECK-LABEL: define goabiinternal <32 x i8> @vec256_add
; CHECK-SAME: #[[AVX2:[0-9]+]]
; CHECK-LABEL: define goabiinternal <64 x i8> @vec512_add
; CHECK-SAME: #[[AVX512:[0-9]+]]
; CHECK-DAG: attributes #[[AVX2]] = { {{.*}}"target-features"="+avx,+avx2"{{.*}} }
; CHECK-DAG: attributes #[[AVX512]] = { {{.*}}"target-features"="+avx,+avx2,+avx512f,+avx512cd,+avx512bw,+avx512dq,+avx512vl"{{.*}} }
; CHECK-NOT: goallc.cpu.feature-floor
; CHECK-NOT: goallc.fmv.slot
; CHECK: !goallc.cpu.fmv.done = !{![[DONE:[0-9]+]]}
