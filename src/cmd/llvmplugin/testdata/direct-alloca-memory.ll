target triple = "aarch64-unknown-linux-goobj"

; IR-LABEL: define goabiinternal void @direct_alloca_stores_across_calls()
; IR: @llvm.experimental.gc.statepoint
; IR: br label %entry.statepoint.cont
; IR: entry.statepoint.cont:
; IR: %slot.relocated{{[0-9]*}} = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %first.remat = getelementptr inbounds i8, ptr %slot.relocated{{[0-9]*}}, i64 16
; IR: store <2 x ptr> %first.error, ptr %first.remat
; IR: @llvm.experimental.gc.statepoint
; IR: br label %entry.statepoint.cont.statepoint.cont
; IR: entry.statepoint.cont.statepoint.cont:
; IR: %slot.relocated{{[0-9]*}} = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %second.remat = getelementptr inbounds i8, ptr %slot.relocated{{[0-9]*}}, i64 48
; IR: store <2 x ptr> %second.error, ptr %second.remat
; IR-NOT: %first.relocated.merge
; IR-NOT: %second.relocated.merge

; MIR-LABEL: name: direct_alloca_stores_across_calls
; MIR: bb.0.entry:
; MIR-NOT: ADDXri %stack.0.slot
; MIR: STATEPOINT
; MIR: bb.1.entry.statepoint.cont:
; MIR: STRQui {{.*}}{{%ir.first.remat}}
; MIR: STATEPOINT
; MIR: bb.2.entry.statepoint.cont.statepoint.cont:
; MIR: STRQui {{.*}}{{%ir.second.remat}}

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

; CodeGenPrepare can share one large alloca-derived base between a direct
; memory use and further GEPs. Even when the further GEPs are formed after the
; call, their shared base must not remain live across the statepoint.
;
; IR-LABEL: define goabiinternal void @direct_alloca_shared_base_across_call()
; IR: @llvm.experimental.gc.statepoint
; IR: br label %entry.statepoint.cont
; IR: entry.statepoint.cont:
; IR: %shared.remat{{[0-9]*}} = getelementptr inbounds i8, ptr %slot, i64 2376
; IR: store <2 x ptr> %value.relocated{{[0-9]*}}, ptr %shared.remat{{[0-9]*}}
; IR: %shared.remat{{[0-9]*}} = getelementptr inbounds i8, ptr %slot, i64 2376
; IR: %child.remat{{[0-9]*}} = getelementptr inbounds i8, ptr %shared.remat{{[0-9]*}}, i64 800
; IR: store <2 x ptr> %value.relocated{{[0-9]*}}, ptr %child.remat{{[0-9]*}}
;
; MIR-LABEL: name: direct_alloca_shared_base_across_call
; MIR: bb.0.entry:
; MIR-NOT: ADDXri %stack.0.slot
; MIR: STATEPOINT
; MIR: bb.1.entry.statepoint.cont:
; MIR: STRQui {{.*}}{{%ir.shared.remat}}
; MIR: STRQui {{.*}}{{%ir.child.remat}}

define goabiinternal void @direct_alloca_shared_base_across_call() gc "goallc" {
entry:
  %slot = alloca [12000 x i8], align 16
  %shared = getelementptr inbounds i8, ptr %slot, i64 2376
  %value = load <2 x ptr>, ptr @error, align 8
  call goabiinternal void @safepoint()
  store <2 x ptr> %value, ptr %shared, align 8
  %child = getelementptr inbounds i8, ptr %shared, i64 800
  store <2 x ptr> %value, ptr %child, align 8
  ret void
}
