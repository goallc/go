target triple = "aarch64-unknown-linux-gnu"

@runtime.goallcCPUFeatures = external global i64
@runtime.arm64HasATOMICS = external global i8

; CHECK: @add.goallc.fmv.slot = internal global ptr null
; CHECK-LABEL: define i64 @add(
; CHECK-SAME: #[[DISPATCH:[0-9]+]]
; CHECK: and i64 %features, 256
; CHECK: select i1 {{.*}}, ptr @add.goallc.fmv.lse, ptr @add.goallc.fmv.baseline

define i64 @add(ptr %address, i64 %delta) #0 {
entry:
  %flag = load i8, ptr @runtime.arm64HasATOMICS, align 1, !goallc.cpu.guard !1
  %enabled = icmp ne i8 %flag, 0
  br i1 %enabled, label %feature, label %fallback

feature:
  %old = atomicrmw add ptr %address, i64 %delta seq_cst, align 8, !goallc.cpu.requires !1
  br label %done

fallback:
  %soft.old = atomicrmw add ptr %address, i64 %delta seq_cst, align 8
  br label %done

done:
  %result = phi i64 [ %old, %feature ], [ %soft.old, %fallback ]
  ret i64 %result
}

; CHECK-LABEL: define internal i64 @add.goallc.fmv.baseline(
; CHECK-NOT: !goallc.cpu.requires
; CHECK: %soft.old = atomicrmw add

; CHECK-LABEL: define internal i64 @add.goallc.fmv.lse(
; CHECK-SAME: #[[LSE:[0-9]+]]
; CHECK: %old = atomicrmw add
; CHECK-NOT: %soft.old = atomicrmw add

; CHECK: attributes #[[DISPATCH]] = {{.*}}noinline
; CHECK: attributes #[[LSE]] = {{.*}}"target-features"="+lse"

attributes #0 = { "goallc.cpu.multiversion"="arm64.lse" "target-cpu"="generic" }

!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"arm64", !"v8.0"}
!1 = !{!"arm64.lse"}
