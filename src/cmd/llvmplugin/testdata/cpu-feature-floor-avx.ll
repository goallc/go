; RUN: opt -load-pass-plugin=GoALLCStatepoints -passes=goallc-cpu-features -S %s -o - | FileCheck %s

target triple = "x86_64-unknown-linux-goobj"

define goabiinternal <16 x i8> @vec_add(<16 x i8> %x, <16 x i8> %y) #0 {
entry:
  %sum = add <16 x i8> %x, %y
  ret <16 x i8> %sum
}

attributes #0 = { "goallc.cpu.feature-floor"="x86.avx" }

!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}

; CHECK: define goabiinternal <16 x i8> @vec_add
; CHECK-SAME: #[[ATTR:[0-9]+]]
; CHECK: attributes #[[ATTR]] = { {{.*}}"target-features"="+avx"{{.*}} }
; CHECK-NOT: goallc.cpu.feature-floor
; CHECK: !goallc.cpu.fmv.done = !{![[DONE:[0-9]+]]}
