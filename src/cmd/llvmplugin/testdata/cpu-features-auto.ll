; No business flag is interpreted as a hardware predicate by automatic FMV.
; The program guarantees the unsupported operation is unreachable in baseline.
target triple = "x86_64-unknown-linux-goobj"
@algorithm.enabled = external global i1
@cpu.avx = external global i1
@runtime.goallcCPUFeatures = external global i64
declare void @ordinary()
declare void @stop() noreturn
declare void @llvm.sideeffect()
declare <2 x i64> @llvm.x86.aesni.aesenc(<2 x i64>, <2 x i64>)

define goabiinternal void @conditional(ptr %x, ptr %out) #0 gc "goallc" {
entry:
  %flag = load i1, ptr @algorithm.enabled
  br i1 %flag, label %feature, label %fallback
feature:
  call void @ordinary()
  call void @llvm.sideeffect(), !goallc.cpu.requires !1, !goallc.cpu.require-anchor !2, !goallc.cpu.auto !2
  %a = load <32 x i8>, ptr %x, align 1
  %sum = add <32 x i8> %a, %a
  call void @ordinary()
  br label %done
fallback:
  br label %done
done:
  %result = phi <32 x i8> [ %sum, %feature ], [ zeroinitializer, %fallback ]
  store <32 x i8> %result, ptr %out, align 1
  ret void
}
attributes #0 = { "target-cpu"="x86-64" "goallc.cpu.multiversion"="x86.avx2" }

; AVX preparation followed by AVX+AES needs no separate AVX-only body.
define goabiinternal void @composite(ptr %x) #1 {
entry:
  call void @llvm.sideeffect(), !goallc.cpu.requires !4, !goallc.cpu.require-anchor !2, !goallc.cpu.auto !2
  %prepared = load <2 x i64>, ptr %x, align 1
  call void @llvm.sideeffect(), !goallc.cpu.requires !3, !goallc.cpu.require-anchor !2, !goallc.cpu.auto !2
  %r = call <2 x i64> @llvm.x86.aesni.aesenc(<2 x i64> %prepared, <2 x i64> %prepared)
  store <2 x i64> %r, ptr %x, align 1
  ret void
}
attributes #1 = { "target-cpu"="x86-64" "goallc.cpu.multiversion"="x86.avx,x86.aes" }
!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}
!1 = !{!"x86.avx2"}
!2 = !{}
!3 = !{!"x86.avxaes"}
!4 = !{!"x86.avx"}

; CHECK-NOT: @dead.goallc.fmv.slot
; CHECK-LABEL: define goabiinternal i32 @after_stop(
; CHECK: call void @ordinary()
; CHECK-NEXT: call void @stop()
; CHECK-NEXT: unreachable
; CHECK-NOT: call void @ordinary()
; CHECK: ret i32 1
; CHECK-LABEL: define goabiinternal void @dead(
; CHECK: call void @stop()
; CHECK-NEXT: unreachable
; CHECK-LABEL: define internal goabiinternal void @"conditional<goallc.fmv.baseline>"(
; CHECK: load i1, ptr @algorithm.enabled
; CHECK: br i1 %flag
; CHECK: call void @ordinary()
; CHECK-NEXT: unreachable
; CHECK-NOT: add <32 x i8>
; CHECK: store <32 x i8> zeroinitializer
; CHECK: ret void
; CHECK-LABEL: define internal goabiinternal void @"conditional<goallc.fmv.avx2>"(
; CHECK-SAME: #[[ISA:[0-9]+]] gc "goallc"
; CHECK: load i1, ptr @algorithm.enabled
; CHECK: br i1 %flag
; CHECK: call void @ordinary()
; CHECK: add <32 x i8>
; CHECK: call void @ordinary()
; CHECK: ret void
; CHECK-LABEL: define internal goabiinternal void @"composite<goallc.fmv.baseline>"(
; CHECK: unreachable
; CHECK-NOT: define {{.*}} @"composite<goallc.fmv.avx>"
; CHECK-NOT: define {{.*}} @"composite<goallc.fmv.aes>"
; CHECK-LABEL: define internal goabiinternal void @"composite<goallc.fmv.avxaes>"(
; CHECK: call <2 x i64> @llvm.x86.aesni.aesenc(
; CHECK: ret void
; CHECK-LABEL: define internal goabiinternal i1 @"observed<goallc.fmv.baseline>"(
; CHECK: ret i1 false
; CHECK-LABEL: define internal goabiinternal i1 @"observed<goallc.fmv.avx>"(
; CHECK: ret i1 true
; CHECK-NOT: define {{.*}} @"observed<goallc.fmv.aes>"
; CHECK-LABEL: define internal goabiinternal i1 @"observed<goallc.fmv.avxaes>"(
; CHECK: call <2 x i64> @llvm.x86.aesni.aesenc(
; CHECK: ret i1 true
; CHECK-NOT: define {{.*}} @"observed<goallc.fmv.avx-avxaes>"

; LLVM's noreturn cleanup must run before checking even non-FMV requirements.
; Preserve preceding effects, remove the dead tail, and repair the join's PHI.
define goabiinternal i32 @after_stop(i1 %exit) {
entry:
  br i1 %exit, label %stop, label %done
stop:
  call void @ordinary()
  call void @stop()
  call void @ordinary()
  call void @llvm.sideeffect(), !goallc.cpu.requires !1, !goallc.cpu.require-anchor !2
  br label %done
done:
  %result = phi i32 [ 1, %entry ], [ 2, %stop ]
  ret i32 %result
}

; A dead automatic requirement must not leave any dispatch machinery behind.
define goabiinternal void @dead() #0 {
  call void @stop()
  call void @llvm.sideeffect(), !goallc.cpu.requires !1, !goallc.cpu.require-anchor !2, !goallc.cpu.auto !2
  ret void
}

; An independent Go feature observation still distinguishes the AVX-only case.
define goabiinternal i1 @observed(i1 %run, ptr %x) #1 {
entry:
  %avx = load i1, ptr @cpu.avx, !goallc.cpu.guard !4
  br i1 %run, label %feature, label %done
feature:
  call void @llvm.sideeffect(), !goallc.cpu.requires !3, !goallc.cpu.require-anchor !2, !goallc.cpu.auto !2
  %a = load <2 x i64>, ptr %x, align 1
  %r = call <2 x i64> @llvm.x86.aesni.aesenc(<2 x i64> %a, <2 x i64> %a)
  store <2 x i64> %r, ptr %x, align 1
  br label %done
done:
  ret i1 %avx
}
; CHECK: attributes #[[ISA]] = { "target-cpu"="x86-64" "target-features"="+avx,+avx2" }
