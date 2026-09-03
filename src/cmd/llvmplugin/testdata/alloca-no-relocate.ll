target triple = "aarch64-unknown-linux-goobj"

; IR-LABEL: define goabiinternal ptr @pointer_contents(
; IR: %slot = alloca ptr
; IR: @llvm.experimental.gc.statepoint{{.*}}"deopt"({{.*}}ptr %slot{{.*}}){{.*}}"gc-live"(ptr %slot)
; IR-NOT: = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %result = load ptr, ptr %slot

; MIR-LABEL: name: pointer_contents
; MIR: fixedStack:
; MIR: - { id: 0,
; MIR: stack: []
; MIR-NOT: %{{(fixed-)?}}stack.1
; MIR-NOT: PHI
; MIR: STATEPOINT{{.*}}%fixed-stack.0
; MIR: bb.1.entry.statepoint.cont:

; IR-LABEL: define goabiinternal void @same_alloca_phi(
; IR-NOT: phi ptr
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %slot)
; IR-NOT: = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %address.remat = getelementptr i8, ptr %slot, i64 0
; IR-NEXT: store ptr null, ptr %address.remat

; MIR-LABEL: name: same_alloca_phi
; MIR: stack:
; MIR: - { id: 0, name: slot,
; MIR-NOT: %{{(fixed-)?}}stack.1
; MIR-NOT: PHI
; MIR: STATEPOINT{{.*}}%stack.0.slot

; IR-LABEL: define goabiinternal i64 @same_alloca_phi_ptrtoint(
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %slot)
; IR-NOT: = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %address.remat = getelementptr i8, ptr %slot, i64 0
; IR-NEXT: %address.int = ptrtoint ptr %address.remat to i64
; IR-NEXT: ret i64 %address.int

; IR-LABEL: define goabiinternal void @derived_from_same_alloca_phi(
; IR-NOT: phi ptr
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %slot)
; IR-NOT: = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %element.idx = mul nsw i64 %index, 8
; IR-NEXT: %element.remat = getelementptr i8, ptr %slot, i64 %element.idx
; IR: @llvm.experimental.gc.statepoint{{.*}}ptr %element.remat
; IR-NOT: = call coldcc ptr @llvm.experimental.gc.relocate

; IR-LABEL: define goabiinternal void @alloca_addrspacecast(
; IR: %address = addrspacecast ptr %slot to ptr addrspace(1)
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"({{.*}}ptr addrspace(1) %address
; IR: %address.relocated = call coldcc ptr addrspace(1) @llvm.experimental.gc.relocate
; IR: store ptr null, ptr addrspace(1) %address.relocated

; IR-LABEL: define goabiinternal void @mixed_alloca_phi(
; IR: %address = phi ptr
; IR: "gc-live"(ptr %address)
; IR: %address.relocated = call coldcc ptr @llvm.experimental.gc.relocate
; IR: store ptr null, ptr %address.relocated

; IR-LABEL: define goabiinternal void @same_alloca_different_offset_phi(
; IR-NOT: phi ptr
; IR: %address.offset = phi i64 [ 0, %left ], [ 8, %right ]
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %slot)
; IR-NOT: = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %address.remat = getelementptr i8, ptr %slot, i64 %address.offset
; IR-NEXT: store ptr null, ptr %address.remat

; IR-LABEL: define goabiinternal void @same_alloca_different_offset_select(
; IR-NOT: select i1 %choose, ptr
; IR: %address.offset = select i1 %choose, i64 0, i64 8
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %slot)
; IR-NOT: = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %address.remat = getelementptr i8, ptr %slot, i64 %address.offset
; IR-NEXT: store ptr null, ptr %address.remat

; IR-LABEL: define goabiinternal void @derived_dynamic(
; IR: %element.idx = mul nsw i64 %index, 8
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %slot)
; IR-NOT: = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %element.remat = getelementptr i8, ptr %slot, i64 %element.idx
; IR-NEXT: store ptr null, ptr %element.remat

; MIR-LABEL: name: derived_dynamic
; MIR: stack:
; MIR: - { id: 0, name: slot,
; MIR-NOT: %{{(fixed-)?}}stack.1
; MIR-NOT: PHI
; MIR: STATEPOINT{{.*}}%stack.0.slot
; MIR: bb.1.entry.statepoint.cont:

; SLP can combine scalar field addresses from one stack object into a fixed
; vector GEP. The vector is still only a recipe for the fixed frame address:
; keep the scalar alloca as the sole stack-map root and rebuild the vector
; immediately at its post-statepoint use.
; IR-LABEL: define goabiinternal void @vector_gep_from_scalar_alloca(
; IR: %slot = alloca [8 x ptr]
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %slot)
; IR-NOT: = call coldcc {{.*}} @llvm.experimental.gc.relocate
; IR: %addresses.remat = getelementptr i8, ptr %slot, <2 x i64> <i64 48, i64 56>
; IR-NEXT: store <2 x ptr> %addresses.remat, ptr @vector_address_sink

; MIR-LABEL: name: vector_gep_from_scalar_alloca
; MIR: stack:
; MIR: - { id: 0, name: slot,
; MIR-NOT: %{{(fixed-)?}}stack.1
; MIR: STATEPOINT{{.*}}%stack.0.slot
; MIR: bb.1.entry.statepoint.cont:

; An integer offset can itself be the result of the statepointed call. Its
; saved recipe must follow call RAUW to gc.result instead of retaining a raw
; pointer to the erased ordinary call.
; IR-LABEL: define goabiinternal void @call_result_dynamic_offset(
; IR: [[TOKEN:%.*]] = call goabiinternal token {{.*}}@llvm.experimental.gc.statepoint
; IR: [[OFFSET:%.*]] = call i64 @llvm.experimental.gc.result.i64(token [[TOKEN]])
; IR-NOT: = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %address.remat = getelementptr i8, ptr %slot, i64 [[OFFSET]]
; IR: @llvm.experimental.gc.statepoint{{.*}}ptr %address.remat

; IR-LABEL: define goabiinternal void @aggregate_leaf_loop(
; IR-NOT: phi ptr
; IR: loop:
; IR: store ptr null, ptr %slot
; IR: %slot.address.remat{{.*}} = getelementptr i8, ptr %slot, i64 0
; IR: @llvm.experimental.gc.statepoint{{.*}}"deopt"({{.*}}ptr %slot{{.*}}){{.*}}"gc-live"(ptr %slot)
; IR-NOT: = call coldcc ptr @llvm.experimental.gc.relocate

; MIR-LABEL: name: aggregate_leaf_loop
; MIR: stack:
; MIR: - { id: 0, name: slot,
; MIR-NOT: %{{(fixed-)?}}stack.1
; MIR-NOT: PHI
; MIR: bb.1.loop:
; MIR: STATEPOINT{{.*}}%stack.0.slot

; IR-LABEL: define goabiinternal void @aggregate_phi_same_alloca(
; IR-NOT: = call coldcc ptr @llvm.experimental.gc.relocate
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %slot)
; IR: merge.statepoint.cont:
; IR: %merged.leaf.0.remat = getelementptr i8, ptr %slot, i64 0
; IR: insertvalue { ptr, i64, i64 } poison, ptr %merged.leaf.0.remat, 0
; IR: @llvm.experimental.gc.statepoint

; MIR-LABEL: name: aggregate_phi_same_alloca
; MIR: stack:
; MIR: - { id: 0, name: slot,
; MIR-NOT: %{{(fixed-)?}}stack.1
; MIR: STATEPOINT{{.*}}%stack.0.slot
; MIR: bb.{{[0-9]+}}.{{.*}}statepoint.cont:
; MIR: {{(LEA64r|ADDXri)}} %stack.0.slot

; IR-LABEL: define goabiinternal void @aggregate_phi_different_offset(
; IR: %merged.offset = phi i64 [ 0, %left ], [ 8, %right ]
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %slot)
; IR-NOT: = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %merged.leaf.0.remat = getelementptr i8, ptr %slot, i64 %merged.offset
; IR: insertvalue { ptr } poison, ptr %merged.leaf.0.remat, 0

; IR-LABEL: define goabiinternal void @nested_aggregate_same_alloca(
; IR-NOT: = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %forwarded.offset = freeze i64 0
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %slot)
; IR: {{.*}}statepoint.cont:
; IR: %nested.leaf.0.0.remat = getelementptr i8, ptr %slot, i64 %forwarded.offset
; IR: insertvalue { { ptr, i64 }, i64 } poison, ptr %nested.leaf.0.0.remat, 0, 0

; IR-LABEL: define goabiinternal void @nested_aggregate_mixed_alloca(
; IR: %nested.leaf.0.0 = extractvalue { { ptr, i64 }, i64 } %nested, 0, 0
; IR: "gc-live"(ptr %nested.leaf.0.0)
; IR: %nested.leaf.0.0.relocated = call coldcc ptr @llvm.experimental.gc.relocate

; IR-LABEL: define goabiinternal void @nested_aggregate_different_offset(
; IR: %selected.offset = select i1 %choose, i64 0, i64 8
; IR: %forwarded.offset = freeze i64 %selected.offset
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %slot)
; IR-NOT: = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %nested.leaf.0.0.remat = getelementptr i8, ptr %slot, i64 %forwarded.offset
; IR: insertvalue { { ptr, i64 }, i64 } poison, ptr %nested.leaf.0.0.remat, 0, 0

declare goabiinternal void @safepoint()
declare goabiinternal void @observe(ptr)
declare goabiinternal i64 @dynamic_offset(ptr)
declare goabiinternal void @consume_slice({ ptr, i64, i64 })
declare goabiinternal void @consume_pointer_aggregate({ ptr })
declare goabiinternal void @consume_nested_aggregate({ { ptr, i64 }, i64 })

@vector_address_sink = external global <2 x ptr>

define goabiinternal ptr @pointer_contents(ptr %value) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %value, ptr %slot, align 8
  call goabiinternal void @safepoint()
  %result = load ptr, ptr %slot, align 8
  ret ptr %result
}

define goabiinternal void @same_alloca_phi(i1 %choose) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr null, ptr %slot, align 8
  br i1 %choose, label %left, label %right

left:
  %left.address = getelementptr inbounds i8, ptr %slot, i64 0
  br label %merge

right:
  %right.address = getelementptr inbounds i8, ptr %slot, i64 0
  br label %merge

merge:
  %address = phi ptr [ %left.address, %left ], [ %right.address, %right ]
  call goabiinternal void @safepoint()
  store ptr null, ptr %address, align 8
  ret void
}

define goabiinternal i64 @same_alloca_phi_ptrtoint(i1 %choose) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr null, ptr %slot, align 8
  br i1 %choose, label %left, label %right

left:
  %left.address = getelementptr inbounds i8, ptr %slot, i64 0
  br label %merge

right:
  %right.address = getelementptr inbounds i8, ptr %slot, i64 0
  br label %merge

merge:
  %address = phi ptr [ %left.address, %left ], [ %right.address, %right ]
  call goabiinternal void @safepoint()
  %address.int = ptrtoint ptr %address to i64
  ret i64 %address.int
}

define goabiinternal void @derived_from_same_alloca_phi(
    i1 %choose, i64 %index) gc "goallc" {
entry:
  %slot = alloca [4 x ptr], align 8
  store [4 x ptr] zeroinitializer, ptr %slot, align 8
  br i1 %choose, label %left, label %right

left:
  %left.address = getelementptr inbounds i8, ptr %slot, i64 0
  br label %merge

right:
  %right.address = getelementptr inbounds i8, ptr %slot, i64 0
  br label %merge

merge:
  %address = phi ptr [ %left.address, %left ], [ %right.address, %right ]
  call goabiinternal void @safepoint()
  %element = getelementptr inbounds [4 x ptr], ptr %address, i64 0, i64 %index
  call goabiinternal void @observe(ptr %element)
  ret void
}

define goabiinternal void @alloca_addrspacecast() gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr null, ptr %slot, align 8
  %address = addrspacecast ptr %slot to ptr addrspace(1)
  call goabiinternal void @safepoint()
  store ptr null, ptr addrspace(1) %address, align 8
  ret void
}

define goabiinternal void @mixed_alloca_phi(i1 %choose) gc "goallc" {
entry:
  %left.slot = alloca ptr, align 8
  %right.slot = alloca ptr, align 8
  store ptr null, ptr %left.slot, align 8
  store ptr null, ptr %right.slot, align 8
  br i1 %choose, label %left, label %right

left:
  %left.address = getelementptr inbounds i8, ptr %left.slot, i64 0
  br label %merge

right:
  %right.address = getelementptr inbounds i8, ptr %right.slot, i64 0
  br label %merge

merge:
  %address = phi ptr [ %left.address, %left ], [ %right.address, %right ]
  call goabiinternal void @safepoint()
  store ptr null, ptr %address, align 8
  ret void
}

define goabiinternal void @same_alloca_different_offset_phi(i1 %choose) gc "goallc" {
entry:
  %slot = alloca [2 x ptr], align 8
  store [2 x ptr] zeroinitializer, ptr %slot, align 8
  br i1 %choose, label %left, label %right

left:
  %first = getelementptr inbounds [2 x ptr], ptr %slot, i64 0, i64 0
  br label %merge

right:
  %second = getelementptr inbounds [2 x ptr], ptr %slot, i64 0, i64 1
  br label %merge

merge:
  %address = phi ptr [ %first, %left ], [ %second, %right ]
  call goabiinternal void @safepoint()
  store ptr null, ptr %address, align 8
  ret void
}

define goabiinternal void @same_alloca_different_offset_select(i1 %choose) gc "goallc" {
entry:
  %slot = alloca [2 x ptr], align 8
  store [2 x ptr] zeroinitializer, ptr %slot, align 8
  %first = getelementptr inbounds [2 x ptr], ptr %slot, i64 0, i64 0
  %second = getelementptr inbounds [2 x ptr], ptr %slot, i64 0, i64 1
  %address = select i1 %choose, ptr %first, ptr %second
  call goabiinternal void @safepoint()
  store ptr null, ptr %address, align 8
  ret void
}

define goabiinternal void @derived_dynamic(i64 %index) gc "goallc" {
entry:
  %slot = alloca [4 x ptr], align 8
  %element = getelementptr inbounds [4 x ptr], ptr %slot, i64 0, i64 %index
  call goabiinternal void @safepoint()
  store ptr null, ptr %element, align 8
  ret void
}

define goabiinternal void @vector_gep_from_scalar_alloca() gc "goallc" {
entry:
  %slot = alloca [8 x ptr], align 8
  store [8 x ptr] zeroinitializer, ptr %slot, align 8
  %addresses = getelementptr inbounds i8, ptr %slot,
      <2 x i64> <i64 48, i64 56>
  call goabiinternal void @safepoint()
  store <2 x ptr> %addresses, ptr @vector_address_sink, align 8
  ret void
}

define goabiinternal void @call_result_dynamic_offset() gc "goallc" {
entry:
  %slot = alloca [32 x i8], align 1
  %offset = call goabiinternal i64 @dynamic_offset(ptr %slot)
  %address = getelementptr inbounds i8, ptr %slot, i64 %offset
  call goabiinternal void @observe(ptr %address)
  ret void
}

define goabiinternal void @aggregate_leaf_loop(i1 %again) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  %slot.address = getelementptr inbounds i8, ptr %slot, i64 0
  %slice.0 = insertvalue { ptr, i64, i64 } poison, ptr %slot.address, 0
  %slice.1 = insertvalue { ptr, i64, i64 } %slice.0, i64 1, 1
  %slice = insertvalue { ptr, i64, i64 } %slice.1, i64 1, 2
  br label %loop

loop:
  store ptr null, ptr %slot, align 8
  call goabiinternal void @consume_slice({ ptr, i64, i64 } %slice)
  br i1 %again, label %loop, label %exit

exit:
  ret void
}

define goabiinternal void @aggregate_phi_same_alloca(i1 %choose) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  br i1 %choose, label %left, label %right

left:
  %left.address = getelementptr inbounds i8, ptr %slot, i64 0
  %left.0 = insertvalue { ptr, i64, i64 } poison, ptr %left.address, 0
  %left.1 = insertvalue { ptr, i64, i64 } %left.0, i64 1, 1
  %left.slice = insertvalue { ptr, i64, i64 } %left.1, i64 1, 2
  br label %merge

right:
  %right.address = getelementptr inbounds i8, ptr %slot, i64 0
  %right.0 = insertvalue { ptr, i64, i64 } poison, ptr %right.address, 0
  %right.1 = insertvalue { ptr, i64, i64 } %right.0, i64 1, 1
  %right.slice = insertvalue { ptr, i64, i64 } %right.1, i64 1, 2
  br label %merge

merge:
  %merged = phi { ptr, i64, i64 } [ %left.slice, %left ], [ %right.slice, %right ]
  call goabiinternal void @safepoint()
  call goabiinternal void @consume_slice({ ptr, i64, i64 } %merged)
  ret void
}

define goabiinternal void @aggregate_phi_different_offset(i1 %choose) gc "goallc" {
entry:
  %slot = alloca [2 x ptr], align 8
  br i1 %choose, label %left, label %right

left:
  %first = getelementptr inbounds [2 x ptr], ptr %slot, i64 0, i64 0
  %left.aggregate = insertvalue { ptr } poison, ptr %first, 0
  br label %merge

right:
  %second = getelementptr inbounds [2 x ptr], ptr %slot, i64 0, i64 1
  %right.aggregate = insertvalue { ptr } poison, ptr %second, 0
  br label %merge

merge:
  %merged = phi { ptr } [ %left.aggregate, %left ], [ %right.aggregate, %right ]
  call goabiinternal void @safepoint()
  call goabiinternal void @consume_pointer_aggregate({ ptr } %merged)
  ret void
}

define goabiinternal void @nested_aggregate_same_alloca(i1 %choose) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  %left.address = getelementptr inbounds i8, ptr %slot, i64 0
  %left.inner.0 = insertvalue { ptr, i64 } poison, ptr %left.address, 0
  %left.inner = insertvalue { ptr, i64 } %left.inner.0, i64 1, 1
  %right.address = getelementptr inbounds i8, ptr %slot, i64 0
  %right.inner.0 = insertvalue { ptr, i64 } poison, ptr %right.address, 0
  %right.inner = insertvalue { ptr, i64 } %right.inner.0, i64 1, 1
  %selected = select i1 %choose, { ptr, i64 } %left.inner, { ptr, i64 } %right.inner
  %forwarded = freeze { ptr, i64 } %selected
  %nested.0 = insertvalue { { ptr, i64 }, i64 } poison, { ptr, i64 } %forwarded, 0
  %nested = insertvalue { { ptr, i64 }, i64 } %nested.0, i64 2, 1
  call goabiinternal void @safepoint()
  call goabiinternal void @consume_nested_aggregate({ { ptr, i64 }, i64 } %nested)
  ret void
}

define goabiinternal void @nested_aggregate_mixed_alloca(i1 %choose) gc "goallc" {
entry:
  %left.slot = alloca ptr, align 8
  %right.slot = alloca ptr, align 8
  %left.inner.0 = insertvalue { ptr, i64 } poison, ptr %left.slot, 0
  %left.inner = insertvalue { ptr, i64 } %left.inner.0, i64 1, 1
  %right.inner.0 = insertvalue { ptr, i64 } poison, ptr %right.slot, 0
  %right.inner = insertvalue { ptr, i64 } %right.inner.0, i64 1, 1
  %selected = select i1 %choose, { ptr, i64 } %left.inner, { ptr, i64 } %right.inner
  %forwarded = freeze { ptr, i64 } %selected
  %nested.0 = insertvalue { { ptr, i64 }, i64 } poison, { ptr, i64 } %forwarded, 0
  %nested = insertvalue { { ptr, i64 }, i64 } %nested.0, i64 2, 1
  call goabiinternal void @safepoint()
  call goabiinternal void @consume_nested_aggregate({ { ptr, i64 }, i64 } %nested)
  ret void
}

define goabiinternal void @nested_aggregate_different_offset(i1 %choose) gc "goallc" {
entry:
  %slot = alloca [2 x ptr], align 8
  %first = getelementptr inbounds [2 x ptr], ptr %slot, i64 0, i64 0
  %left.inner.0 = insertvalue { ptr, i64 } poison, ptr %first, 0
  %left.inner = insertvalue { ptr, i64 } %left.inner.0, i64 1, 1
  %second = getelementptr inbounds [2 x ptr], ptr %slot, i64 0, i64 1
  %right.inner.0 = insertvalue { ptr, i64 } poison, ptr %second, 0
  %right.inner = insertvalue { ptr, i64 } %right.inner.0, i64 1, 1
  %selected = select i1 %choose, { ptr, i64 } %left.inner, { ptr, i64 } %right.inner
  %forwarded = freeze { ptr, i64 } %selected
  %nested.0 = insertvalue { { ptr, i64 }, i64 } poison, { ptr, i64 } %forwarded, 0
  %nested = insertvalue { { ptr, i64 }, i64 } %nested.0, i64 2, 1
  call goabiinternal void @safepoint()
  call goabiinternal void @consume_nested_aggregate({ { ptr, i64 }, i64 } %nested)
  ret void
}
