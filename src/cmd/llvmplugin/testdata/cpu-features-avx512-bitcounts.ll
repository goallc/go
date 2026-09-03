target triple = "x86_64-unknown-linux-gnu"

@runtime.goallcCPUFeatures = external global i64
@runtime.x86HasAVX512 = external global i8
@runtime.x86HasAVX512BITALG = external global i8
@runtime.x86HasAVX512VPOPCNTDQ = external global i8

declare i64 @fallback(i64)

; CHECK: @bitalg.goallc.fmv.slot = internal global ptr @"bitalg<goallc.fmv.resolve>"
; CHECK: @combined.goallc.fmv.slot = internal global ptr @"combined<goallc.fmv.resolve>"
; CHECK: @vpopcntdq.goallc.fmv.slot = internal global ptr @"vpopcntdq<goallc.fmv.resolve>"
; CHECK-LABEL: define internal i64 @bitalg(
; CHECK: load atomic ptr, ptr @bitalg.goallc.fmv.slot monotonic
; CHECK: tail call i64 %target
; CHECK-LABEL: define internal i64 @combined(
; CHECK: load atomic ptr, ptr @combined.goallc.fmv.slot monotonic
; CHECK: tail call i64 %target
; CHECK-LABEL: define internal i64 @vpopcntdq(
; CHECK: load atomic ptr, ptr @vpopcntdq.goallc.fmv.slot monotonic
; CHECK: tail call i64 %target

define internal i64 @bitalg(i64 %x) #0 {
entry:
  %flag = load i8, ptr @runtime.x86HasAVX512BITALG, align 1, !goallc.cpu.guard !1
  %enabled = icmp ne i8 %flag, 0
  br i1 %enabled, label %feature, label %fallback

feature:
  %value = add i64 %x, 1, !goallc.cpu.requires !1
  br label %done

fallback:
  %soft = call i64 @fallback(i64 %x)
  br label %done

done:
  %result = phi i64 [ %value, %feature ], [ %soft, %fallback ]
  ret i64 %result
}

define internal i64 @combined(i64 %x) #2 {
entry:
  %avx512flag = load i8, ptr @runtime.x86HasAVX512, align 1, !goallc.cpu.guard !3
  %avx512enabled = icmp ne i8 %avx512flag, 0
  br i1 %avx512enabled, label %avx512, label %fallback

avx512:
  %wide = add i64 %x, 1, !goallc.cpu.requires !3
  br label %join

fallback:
  %soft = call i64 @fallback(i64 %x)
  br label %join

join:
  %value = phi i64 [ %wide, %avx512 ], [ %soft, %fallback ]
  %bitalgflag = load i8, ptr @runtime.x86HasAVX512BITALG, align 1, !goallc.cpu.guard !1
  %bitalgenabled = icmp ne i8 %bitalgflag, 0
  br i1 %bitalgenabled, label %bitalg, label %done

bitalg:
  %count = add i64 %value, 1, !goallc.cpu.requires !1
  ret i64 %count

done:
  ret i64 %value
}

; CHECK-LABEL: define internal i64 @"bitalg<goallc.fmv.baseline>"(
; CHECK-NOT: add i64
; CHECK: call i64 @fallback

; CHECK-LABEL: define internal i64 @"bitalg<goallc.fmv.avx512bitalg>"(
; CHECK-SAME: #[[BITALG:[0-9]+]]
; CHECK: add i64
; CHECK-NOT: call i64 @fallback

; CHECK-LABEL: define internal i64 @"bitalg<goallc.fmv.resolve>"(
; CHECK: and i64 %features, 2048
; CHECK: select i1 {{.*}}, ptr @"bitalg<goallc.fmv.avx512bitalg>", ptr @"bitalg<goallc.fmv.baseline>"

define internal i64 @vpopcntdq(i64 %x) #1 {
entry:
  %flag = load i8, ptr @runtime.x86HasAVX512VPOPCNTDQ, align 1, !goallc.cpu.guard !2
  %enabled = icmp ne i8 %flag, 0
  br i1 %enabled, label %feature, label %fallback

feature:
  %value = add i64 %x, 1, !goallc.cpu.requires !2
  br label %done

fallback:
  %soft = call i64 @fallback(i64 %x)
  br label %done

done:
  %result = phi i64 [ %value, %feature ], [ %soft, %fallback ]
  ret i64 %result
}

; CHECK-LABEL: define internal i64 @"combined<goallc.fmv.baseline>"(
; CHECK-NOT: add i64
; CHECK: call i64 @fallback

; CHECK-LABEL: define internal i64 @"combined<goallc.fmv.avx512>"(
; CHECK-COUNT-1: add i64

; CHECK-LABEL: define internal i64 @"combined<goallc.fmv.avx512bitalg>"(
; CHECK: call i64 @fallback
; CHECK-COUNT-1: add i64

; CHECK-LABEL: define internal i64 @"combined<goallc.fmv.avx512-avx512bitalg>"(
; CHECK-COUNT-2: add i64
; CHECK-NOT: call i64 @fallback

; CHECK-LABEL: define internal i64 @"combined<goallc.fmv.resolve>"(
; CHECK: and i64 %features, 1024
; CHECK: and i64 %features, 2048
; CHECK: and i64 %features, 3072
; CHECK: combined<goallc.fmv.avx512-avx512bitalg>

; CHECK-LABEL: define internal i64 @"vpopcntdq<goallc.fmv.baseline>"(
; CHECK-NOT: add i64
; CHECK: call i64 @fallback

; CHECK-LABEL: define internal i64 @"vpopcntdq<goallc.fmv.avx512vpopcntdq>"(
; CHECK-SAME: #[[VPOPCNTDQ:[0-9]+]]
; CHECK: add i64
; CHECK-NOT: call i64 @fallback

; CHECK-LABEL: define internal i64 @"vpopcntdq<goallc.fmv.resolve>"(
; CHECK: and i64 %features, 4096
; CHECK: select i1 {{.*}}, ptr @"vpopcntdq<goallc.fmv.avx512vpopcntdq>", ptr @"vpopcntdq<goallc.fmv.baseline>"

; CHECK: attributes #[[BITALG]] = {{.*}}"target-features"="+avx,+avx2,+avx512f,+avx512cd,+avx512bw,+avx512dq,+avx512vl,+avx512bitalg"
; CHECK: attributes #[[VPOPCNTDQ]] = {{.*}}"target-features"="+avx,+avx2,+avx512f,+avx512cd,+avx512bw,+avx512dq,+avx512vl,+avx512vpopcntdq"

attributes #0 = { "goallc.cpu.feature-floor"="x86.avx" "goallc.cpu.multiversion"="x86.avx512bitalg" "target-cpu"="x86-64" }
attributes #1 = { "goallc.cpu.multiversion"="x86.avx512vpopcntdq" "target-cpu"="x86-64" }
attributes #2 = { "goallc.cpu.multiversion"="x86.avx512,x86.avx512bitalg" "target-cpu"="x86-64" }

!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}
!1 = !{!"x86.avx512bitalg"}
!2 = !{!"x86.avx512vpopcntdq"}
!3 = !{!"x86.avx512"}
