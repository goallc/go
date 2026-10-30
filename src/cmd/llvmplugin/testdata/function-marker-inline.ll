target triple = "x86_64-unknown-linux-goobj"

; INLINE-OPT-NOT: define {{.*}}@callee(
; INLINE-OPT-LABEL: define goabiinternal void @caller(
; INLINE-OPT: call void @llvm.sideeffect(), !goobj.marker_reloc

; INLINE-REWRITE-NOT: call void @llvm.sideeffect
; INLINE-REWRITE-NOT: define {{.*}}@callee(
; INLINE-REWRITE-LABEL: define goabiinternal void @caller(
; INLINE-REWRITE-SAME: !goobj.marker_relocs ![[MARKERS:[0-9]+]]
; INLINE-REWRITE-NOT: !goobj.marker_relocs =
; INLINE-REWRITE: ![[MARKERS]] = !{![[IFACE:[0-9]+]], ![[FIELD:[0-9]+]], ![[OTHERADD:[0-9]+]], ![[OTHERTYPE:[0-9]+]]}
; INLINE-REWRITE: ![[IFACE]] = {{(distinct )?}}!{ptr @target, i32 24, i64 96}
; INLINE-REWRITE: ![[FIELD]] = !{ptr @"go:track.pkg.T.X", i32 21, i64 0}
; INLINE-REWRITE: ![[OTHERADD]] = !{ptr @target, i32 24, i64 97}
; INLINE-REWRITE: ![[OTHERTYPE]] = !{ptr @target, i32 23, i64 96}

@target = external global i8
@"go:track.pkg.T.X" = external global i8
@llvm.compiler.used = appending global [2 x ptr] [ptr @target, ptr @"go:track.pkg.T.X"], section "llvm.metadata"

declare void @llvm.sideeffect()

define internal goabiinternal void @callee()
    gc "goallc" {
entry:
  call void @llvm.sideeffect(), !goobj.marker_reloc !0
  call void @llvm.sideeffect(), !goobj.marker_reloc !1
  ret void
}

; The existing distinct node and both inlined copies describe the same edge.
; Different addends and types must remain separate.
define goabiinternal void @caller()
    gc "goallc" !goobj.marker_relocs !5 {
entry:
  call goabiinternal void @callee()
  call goabiinternal void @callee()
  call void @llvm.sideeffect(), !goobj.marker_reloc !3
  call void @llvm.sideeffect(), !goobj.marker_reloc !4
  ret void
}

!0 = !{ptr @target, i32 24, i64 96}
!1 = !{ptr @"go:track.pkg.T.X", i32 21, i64 0}
!2 = distinct !{ptr @target, i32 24, i64 96}
!3 = !{ptr @target, i32 24, i64 97}
!4 = !{ptr @target, i32 23, i64 96}
!5 = !{!2, !0}
