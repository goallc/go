target triple = "x86_64-unknown-linux-gnu"

@runtime.goallcCPUFeatures = external global i64
@runtime.x86HasPOPCNT = external global i8

declare i64 @fallback(i64)
declare i64 @llvm.ctpop.i64(i64)

; CHECK: @ones.goallc.fmv.slot = internal global ptr @"ones<goallc.fmv.resolve>"{{.*}}!goobj.symbol.flags ![[DUPOK:[0-9]+]]
; CHECK-LABEL: define internal i64 @ones(
; CHECK-SAME: #[[DISPATCH:[0-9]+]]
; CHECK: load atomic ptr, ptr @ones.goallc.fmv.slot monotonic
; CHECK: tail call i64 %target

define internal i64 @ones(i64 %x) #0 !goobj.symbol.flags !2 {
entry:
  %flag = load i8, ptr @runtime.x86HasPOPCNT, align 1, !goallc.cpu.guard !1
  %enabled = icmp ne i8 %flag, 0
  br i1 %enabled, label %feature, label %fallback

feature:
  %value = call i64 @llvm.ctpop.i64(i64 %x), !goallc.cpu.requires !1
  br label %done

fallback:
  %soft = call i64 @fallback(i64 %x)
  br label %done

done:
  %result = phi i64 [ %value, %feature ], [ %soft, %fallback ]
  ret i64 %result
}

; CHECK-LABEL: define internal i64 @"ones<goallc.fmv.baseline>"(
; CHECK-SAME: !goobj.symbol.flags ![[DUPOK]]
; CHECK-NOT: llvm.ctpop
; CHECK: call i64 @fallback

; CHECK-LABEL: define internal i64 @"ones<goallc.fmv.popcnt>"(
; CHECK-SAME: #[[POPCNT:[0-9]+]]
; CHECK-SAME: !goobj.symbol.flags ![[DUPOK]]
; CHECK: call i64 @llvm.ctpop.i64
; CHECK-NOT: call i64 @fallback

; CHECK-LABEL: define internal i64 @"ones<goallc.fmv.resolve>"(
; CHECK-SAME: !goobj.symbol.flags ![[DUPOK]]
; CHECK: load i64, ptr @runtime.goallcCPUFeatures
; CHECK: and i64 %features, 128
; CHECK: select i1 {{.*}}, ptr @"ones<goallc.fmv.popcnt>", ptr @"ones<goallc.fmv.baseline>"
; CHECK: store atomic ptr {{.*}}, ptr @ones.goallc.fmv.slot monotonic

; CHECK: attributes #[[DISPATCH]] = {{.*}}"go-nosplit" {{.*}}"target-cpu"="x86-64"
; CHECK: attributes #[[POPCNT]] = {{.*}}"target-features"="+popcnt"
; CHECK: ![[DUPOK]] = !{i32 1, i32 0}

attributes #0 = { "goallc.cpu.multiversion"="x86.popcnt" "target-cpu"="x86-64" }

!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}
!1 = !{!"x86.popcnt"}
!2 = !{i32 1, i32 0}
