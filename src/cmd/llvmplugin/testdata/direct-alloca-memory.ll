target triple = "aarch64-unknown-linux-goobj"

; IR-LABEL: define goabiinternal void @direct_alloca_stores_across_calls()
; IR: @llvm.experimental.gc.statepoint
; IR: br label %entry.statepoint.cont
; IR: entry.statepoint.cont:
; IR: %first.remat = getelementptr inbounds i8, ptr %slot, i64 16
; IR: store <2 x ptr> %first.error, ptr %first.remat
; IR: @llvm.experimental.gc.statepoint
; IR: br label %entry.statepoint.cont.statepoint.cont
; IR: entry.statepoint.cont.statepoint.cont:
; IR: %second.remat = getelementptr inbounds i8, ptr %slot, i64 48
; IR: store <2 x ptr> %second.error, ptr %second.remat
; IR-NOT: %first.relocated.merge
; IR-NOT: %second.relocated.merge

; MIR-LABEL: name: direct_alloca_stores_across_calls
; MIR: bb.0.entry:
; MIR-NOT: ADDXri %stack.0.slot
; MIR: STATEPOINT
; MIR: bb.1.entry.statepoint.cont:
; MIR: STRQui {{.*}}, %stack.0.slot, 1 {{.*}}%ir.first.remat
; MIR: STATEPOINT
; MIR: bb.2.entry.statepoint.cont.statepoint.cont:
; MIR: STRQui {{.*}}, %stack.0.slot, 3 {{.*}}%ir.second.remat

declare goabiinternal void @safepoint()
@error = external global <2 x ptr>

define goabiinternal void @direct_alloca_stores_across_calls() gc "goallc" {
entry:
  %slot = alloca [2 x { ptr, ptr }], align 8
  %first = getelementptr inbounds i8, ptr %slot, i64 16
  %second = getelementptr inbounds i8, ptr %slot, i64 48
  call goabiinternal void @safepoint()
  %first.error = load <2 x ptr>, ptr @error, align 8
  store <2 x ptr> %first.error, ptr %first, align 8
  call goabiinternal void @safepoint()
  %second.error = load <2 x ptr>, ptr @error, align 8
  store <2 x ptr> %second.error, ptr %second, align 8
  ret void
}
