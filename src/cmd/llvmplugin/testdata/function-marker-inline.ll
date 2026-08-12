target triple = "x86_64-unknown-linux-goobj"

; INLINE-OPT-NOT: define {{.*}}@callee(
; INLINE-OPT-LABEL: define goabiinternal void @caller(
; INLINE-OPT: call void @llvm.sideeffect(), !goobj.marker_reloc

; INLINE-REWRITE-NOT: call void @llvm.sideeffect
; INLINE-REWRITE-NOT: !{ptr @callee, ptr @target, i32 24, i64 96}
; INLINE-REWRITE: !{ptr @caller, ptr @target, i32 24, i64 96}

@target = external global i8
@llvm.compiler.used = appending global [1 x ptr] [ptr @target], section "llvm.metadata"

declare void @llvm.sideeffect()

define internal goabiinternal void @callee()
    gc "goallc" {
entry:
  call void @llvm.sideeffect(), !goobj.marker_reloc !0
  ret void
}

define goabiinternal void @caller()
    gc "goallc" {
entry:
  call goabiinternal void @callee()
  ret void
}

!0 = !{ptr @target, i32 24, i64 96}
