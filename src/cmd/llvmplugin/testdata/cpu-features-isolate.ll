; An ordinary algorithm flag is not a hardware predicate. Preserve it and
; isolate the feature-specific code behind a memory ABI, including live-outs.
target triple = "x86_64-unknown-linux-goobj"
@algorithm.enabled = external global i1
@runtime.goallcCPUFeatures = external global i64
@cpu.avx2 = external global i1
declare void @ordinary()

define goabiinternal void @conditional(ptr %x, ptr %y, ptr %out) #0 gc "goallc" {
entry:
  %flag = load i1, ptr @algorithm.enabled
  br i1 %flag, label %feature, label %fallback
feature:
  call void @ordinary()
  %a = load <32 x i8>, ptr %x, align 1
  %b = load <32 x i8>, ptr %y, align 1
  %sum = add <32 x i8> %a, %b, !goallc.cpu.requires !1, !goallc.cpu.outline !1
  call void @ordinary()
  br label %done
fallback:
  br label %done
done:
  %result = phi <32 x i8> [ %sum, %feature ], [ zeroinitializer, %fallback ]
  store <32 x i8> %result, ptr %out, align 1
  ret void
}

; An entry operation without a local guard must not hide the later hardware
; check from FMV. The guarded operation disappears from the baseline clone.
define goabiinternal void @mixed(ptr %x, ptr %out) #1 gc "goallc" {
entry:
  %a = load <32 x i8>, ptr %x, align 1
  %sum = add <32 x i8> %a, %a, !goallc.cpu.requires !1, !goallc.cpu.outline !1
  store <32 x i8> %sum, ptr %out, align 1
  %flag = load i1, ptr @cpu.avx2, !goallc.cpu.guard !1
  br i1 %flag, label %feature, label %done
feature:
  %twice = add <32 x i8> %sum, %sum, !goallc.cpu.requires !1
  store <32 x i8> %twice, ptr %out, align 1
  br label %done
done:
  ret void
}
attributes #0 = { "target-cpu"="x86-64" }
attributes #1 = { "target-cpu"="x86-64" "goallc.cpu.multiversion"="x86.avx2" }
!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}
!1 = !{!"x86.avx2"}

; CHECK-LABEL: define goabiinternal void @conditional(
; CHECK-SAME: #[[BASE:[0-9]+]] gc "goallc"
; CHECK: load i1, ptr @algorithm.enabled
; CHECK: br i1 %flag
; CHECK: call void @ordinary()
; CHECK: call goabiinternal void @conditional.goallc.isa(ptr
; CHECK: call void @ordinary()
; CHECK: store <32 x i8>
; CHECK-LABEL: define internal goabiinternal void @conditional.goallc.isa(ptr
; CHECK-SAME: #[[ISA:[0-9]+]] gc "goallc"
; CHECK: add <32 x i8>
; CHECK-NOT: call void @ordinary()
; CHECK-LABEL: define internal goabiinternal void @"mixed<goallc.fmv.baseline>"(
; CHECK-NOT: load i1, ptr @cpu.avx2
; CHECK-NOT: add <32 x i8>
; CHECK: call goabiinternal void @mixed.goallc.isa.baseline(ptr
; CHECK-NOT: add <32 x i8>
; CHECK: ret void
; CHECK-LABEL: define internal goabiinternal void @"mixed<goallc.fmv.avx2>"(
; CHECK-NOT: load i1, ptr @cpu.avx2
; CHECK: add <32 x i8>
; CHECK: attributes #[[BASE]] = { "target-cpu"="x86-64" }
; CHECK: attributes #[[ISA]] = { noinline "target-cpu"="x86-64" "target-features"="+avx,+avx2" }
