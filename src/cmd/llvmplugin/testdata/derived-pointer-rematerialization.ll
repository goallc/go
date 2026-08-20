target triple = "aarch64-unknown-linux-goobj"

; IR-LABEL: define goabiinternal i1 @hoisted_null_offset(
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %base)
; IR: %base.relocated{{.*}} = call {{.*}}ptr @llvm.experimental.gc.relocate
; IR: %derived.remat{{.*}} = getelementptr i8, ptr %base.relocated{{.*}}, i64 96

; IR-LABEL: define goabiinternal i8 @derived_chain(
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %base)
; IR: %field.remat{{.*}} = getelementptr i8, ptr %base.relocated{{.*}}, i64 16
; IR: %element.remat{{.*}} = getelementptr i8, ptr %field.remat{{.*}}, i64 8

; IR-LABEL: define goabiinternal i8 @conditional_derived(
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %base)
; IR: %derived.relocated.merge{{.*}} = phi ptr [ %derived.remat, %call.statepoint.cont ], [ %derived, %skip ]

; IR-LABEL: define goabiinternal <2 x ptr> @derived_vector(
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(<2 x ptr> %base)
; IR: %base.relocated{{.*}} = call {{.*}}<2 x ptr> @llvm.experimental.gc.relocate.v2p0
; IR: %derived.remat{{.*}} = getelementptr i8, <2 x ptr> %base.relocated{{.*}}, <2 x i64> <i64 16, i64 32>

; IR-LABEL: define goabiinternal <2 x ptr> @derived_vector_from_scalar(
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %base)
; IR: %derived.remat{{.*}} = getelementptr i8, ptr %base.relocated{{.*}}, <2 x i64> <i64 16, i64 32>

declare goabiinternal void @callee()

; Match the optimized form of issue56990: LLVM may legally hoist a field GEP
; above the source nil guard.  The non-heap value null+96 must not become a Go
; GC root at the intervening safepoint.
define goabiinternal i1 @hoisted_null_offset(ptr %base)
    gc "goallc" {
entry:
  %derived = getelementptr i8, ptr %base, i64 96
  call goabiinternal void @callee()
  %base.isnull = icmp eq ptr %base, null
  %derived.isnull = icmp eq ptr %derived, null
  %unused = or i1 %base.isnull, %derived.isnull
  ret i1 %unused
}

; Rebuild the full address-expression chain from the relocated base, not from
; an independently relocated interior pointer.
define goabiinternal i8 @derived_chain(ptr %base)
    gc "goallc" {
entry:
  %field = getelementptr i8, ptr %base, i64 16
  %element = getelementptr i8, ptr %field, i64 8
  call goabiinternal void @callee()
  %value = load i8, ptr %element, align 1
  ret i8 %value
}

; Keep the existing relocation-SSA behavior when only one predecessor crosses
; a safepoint: the merge must select the rebuilt address on that path and the
; original address on the other path.
define goabiinternal i8 @conditional_derived(ptr %base, i1 %take_call)
    gc "goallc" {
entry:
  %derived = getelementptr i8, ptr %base, i64 32
  br i1 %take_call, label %call, label %skip

call:
  call goabiinternal void @callee()
  br label %merge

skip:
  br label %merge

merge:
  %value = load i8, ptr %derived, align 1
  ret i8 %value
}

; A fixed vector of derived pointers has the same provenance rule as a scalar
; derived pointer. Relocate the vector base as one value, then rebuild the
; vector GEP instead of exposing its interior-pointer lanes as Go GC roots.
define goabiinternal <2 x ptr> @derived_vector(<2 x ptr> %base)
    gc "goallc" {
entry:
  %derived = getelementptr i8, <2 x ptr> %base,
      <2 x i64> <i64 16, i64 32>
  call goabiinternal void @callee()
  ret <2 x ptr> %derived
}

; Vector indices can also produce a vector of derived pointers from one scalar
; base. Keep that scalar base live and rebuild the vector result after it is
; relocated.
define goabiinternal <2 x ptr> @derived_vector_from_scalar(ptr %base)
    gc "goallc" {
entry:
  %derived = getelementptr i8, ptr %base,
      <2 x i64> <i64 16, i64 32>
  call goabiinternal void @callee()
  ret <2 x ptr> %derived
}
