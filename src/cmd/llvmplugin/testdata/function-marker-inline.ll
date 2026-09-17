target triple = "x86_64-unknown-linux-goobj"

; INLINE-OPT-NOT: define {{.*}}@callee(
; INLINE-OPT-LABEL: define goabiinternal void @caller(
; INLINE-OPT: call void @llvm.sideeffect(), !goobj.marker_reloc

; INLINE-REWRITE-NOT: call void @llvm.sideeffect
; INLINE-REWRITE-NOT: !{ptr @callee, ptr @target, i32 24, i64 96}
; INLINE-REWRITE: !{ptr @caller, ptr @target, i32 24, i64 96}
; INLINE-REWRITE: !{ptr @caller, ptr @"go:track.pkg.T.X", i32 21, i64 0}

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

define goabiinternal void @caller()
    gc "goallc" {
entry:
  call goabiinternal void @callee()
  ret void
}

!0 = !{ptr @target, i32 24, i64 96}
!1 = !{ptr @"go:track.pkg.T.X", i32 21, i64 0}
