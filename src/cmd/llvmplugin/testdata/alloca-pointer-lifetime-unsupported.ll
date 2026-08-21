target triple = "x86_64-unknown-linux-goobj"

; IR-LABEL: define goabiinternal void @locals_pointer_alloca_with_lifetime()
; IR-NOT: !llvm.stackcoloring.no_merge
; IR: @llvm.experimental.gc.statepoint
; IR: call void @llvm.lifetime.start
; IR: store ptr null
; IR-NOT: call void @llvm.memset.inline
; IR: @llvm.experimental.gc.statepoint{{.*}}"deopt"({{.*}}i64 1095520067{{.*}}){{.*}}"gc-live"(ptr %slot)
; IR: %slot.relocated = call coldcc ptr @llvm.experimental.gc.relocate
; IR: @llvm.experimental.gc.statepoint
; IR: ret void

; IR-LABEL: define goabiinternal void @stack_object_alloca_with_lifetime()
; IR: %slot = alloca ptr, align 8, !llvm.stackcoloring.no_merge
; IR-NOT: "gc-live"(ptr %slot)
; IR: @llvm.experimental.gc.statepoint{{.*}}"deopt"({{.*}}i64 1095520067
; IR: call void @llvm.lifetime.start
; IR: store ptr null
; IR-NOT: call void @llvm.memset.inline
; IR-NOT: "gc-live"(ptr %slot)
; IR: @llvm.experimental.gc.statepoint{{.*}}"deopt"({{.*}}i64 1095520067
; IR-NOT: "gc-live"(ptr %slot)
; IR: @llvm.experimental.gc.statepoint{{.*}}"deopt"({{.*}}i64 1095520067

; IR-LABEL: define goabiinternal void @loop_reinitialized_pointer_alloca(
; IR: call void @llvm.lifetime.start
; IR-NOT: call void @llvm.memset.inline
; IR: @llvm.experimental.gc.statepoint{{.*}}"deopt"({{.*}}i64 1095520067{{.*}}){{.*}}"gc-live"(ptr %slot)
; IR: @llvm.experimental.gc.statepoint
; IR: ret void

; IR-LABEL: define goabiinternal void @preinitialized_pointer_alloca(
; IR: call void @llvm.lifetime.start
; IR-NEXT: call void @llvm.memset.inline
; IR-NOT: call void @llvm.memset.inline
; IR: @llvm.experimental.gc.statepoint

; IR-LABEL: define goabiinternal void @store_initialized_pointer_alloca(
; IR: call void @llvm.lifetime.start
; IR-NOT: call void @llvm.memset.inline
; IR-COUNT-2: store ptr
; IR: i64 3, i64 64, i64 1, i64 5

; IR-LABEL: define goabiinternal void @partially_stored_pointer_alloca(
; IR: call void @llvm.lifetime.start
; IR-NEXT: call void @llvm.memset.inline
; IR-NEXT: %first.field.remat = getelementptr
; IR-NEXT: store ptr %first, ptr %first.field.remat
; IR: @llvm.experimental.gc.statepoint

; IR-LABEL: define goabiinternal void @phi_edge_pointer_alloca(
; IR: %slot = alloca ptr, align 8, !llvm.stackcoloring.no_merge
; IR: %slot.address = getelementptr inbounds i8, ptr %slot, i64 0
; IR: %selected = phi ptr [ %slot.address, %initialize ], [ %other, %external ]
; IR: @llvm.experimental.gc.statepoint{{.*}}"deopt"({{.*}}i64 1095520067{{.*}}){{.*}}"gc-live"(ptr %selected)

; IR-LABEL: define goabiinternal void @hoisted_aggregate_pointer_alloca()
; IR: call void @llvm.lifetime.start
; IR-NEXT: call void @llvm.memset.inline
; IR: %slice.cap.leaf.0 = extractvalue
; IR: @llvm.experimental.gc.statepoint{{.*}}"deopt"({{.*}}i64 1095520067{{.*}}){{.*}}"gc-live"(ptr %slice.cap.leaf.0)
; IR: %slice.cap.leaf.0.relocated
; IR: store ptr null
; IR: @llvm.experimental.gc.statepoint{{.*}}"deopt"({{.*}}i64 1095520067
; IR: @llvm.experimental.gc.statepoint{{.*}}"deopt"({{.*}}i64 1095520067
; IR-NOT: call void @llvm.lifetime.end

; OBJVIEW-LABEL: "name": "stack_object_alloca_with_lifetime"
; OBJVIEW: "kind": "locals_pointer_maps"
; OBJVIEW: "bitmaps": [
; OBJVIEW-NOT: "set_bits": [
; OBJVIEW: "kind": "stack_objects"
; OBJVIEW: "stack_objects": [
; OBJVIEW: "index": 0
; OBJVIEW: "stack_map_queries": [
; OBJVIEW: "stack_map_index": 0
; OBJVIEW: "stack_map_index": 0
; OBJVIEW: "stack_map_index": 0

%pointer_gap = type { ptr, i64, ptr }

declare void @llvm.lifetime.start.p0(i64 immarg, ptr nocapture)
declare void @llvm.lifetime.end.p0(i64 immarg, ptr nocapture)
declare void @llvm.memset.inline.p0.i64(ptr nocapture writeonly, i8, i64, i1 immarg)
declare void @llvm.fake.use(...)
declare goabiinternal void @safepoint()
declare goabiinternal void @observe(ptr)
declare goabiinternal void @observe_slice({ ptr, i64, i64 })

define goabiinternal void @locals_pointer_alloca_with_lifetime() gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  call goabiinternal void @safepoint()
  call void @llvm.lifetime.start.p0(i64 8, ptr %slot)
  store ptr null, ptr %slot, align 8
  call goabiinternal void @safepoint()
  call void (...) @llvm.fake.use(ptr %slot)
  call goabiinternal void @safepoint()
  ret void
}

define goabiinternal void @stack_object_alloca_with_lifetime() gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  call goabiinternal void @safepoint()
  call void @llvm.lifetime.start.p0(i64 8, ptr %slot)
  store ptr null, ptr %slot, align 8
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @safepoint()
  ret void
}

define goabiinternal void @loop_reinitialized_pointer_alloca(i1 %again) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  br label %loop

loop:
  call void @llvm.lifetime.start.p0(i64 8, ptr %slot)
  store ptr null, ptr %slot, align 8
  call goabiinternal void @safepoint()
  call void (...) @llvm.fake.use(ptr %slot)
  br i1 %again, label %loop, label %exit

exit:
  call goabiinternal void @safepoint()
  ret void
}

define goabiinternal void @preinitialized_pointer_alloca() gc "goallc" {
entry:
  %slot = alloca [2 x ptr], align 8
  call void @llvm.lifetime.start.p0(i64 16, ptr %slot)
  call void @llvm.memset.inline.p0.i64(ptr align 8 %slot, i8 0, i64 16, i1 false)
  call goabiinternal void @safepoint()
  call void (...) @llvm.fake.use(ptr %slot)
  ret void
}

define goabiinternal void @store_initialized_pointer_alloca(
    ptr %first, ptr %second) gc "goallc" {
entry:
  %slot = alloca %pointer_gap, align 8
  call void @llvm.lifetime.start.p0(i64 24, ptr %slot)
  %first.field = getelementptr inbounds %pointer_gap, ptr %slot, i32 0, i32 0
  store ptr %first, ptr %first.field, align 8
  %second.field = getelementptr inbounds %pointer_gap, ptr %slot, i32 0, i32 2
  store ptr %second, ptr %second.field, align 8
  call goabiinternal void @safepoint()
  call void (...) @llvm.fake.use(ptr %slot)
  ret void
}

define goabiinternal void @partially_stored_pointer_alloca(
    ptr %first, ptr %second) gc "goallc" {
entry:
  %slot = alloca %pointer_gap, align 8
  call void @llvm.lifetime.start.p0(i64 24, ptr %slot)
  %first.field = getelementptr inbounds %pointer_gap, ptr %slot, i32 0, i32 0
  store ptr %first, ptr %first.field, align 8
  call goabiinternal void @safepoint()
  %second.field = getelementptr inbounds %pointer_gap, ptr %slot, i32 0, i32 2
  store ptr %second, ptr %second.field, align 8
  call void (...) @llvm.fake.use(ptr %slot)
  ret void
}

define goabiinternal void @phi_edge_pointer_alloca(
    i1 %use_stack, ptr %other) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  br i1 %use_stack, label %initialize, label %external

initialize:
  call void @llvm.lifetime.start.p0(i64 8, ptr %slot)
  store ptr null, ptr %slot, align 8
  br label %merge

external:
  br label %merge

merge:
  %selected = phi ptr [ %slot, %initialize ], [ %other, %external ]
  call goabiinternal void @safepoint()
  call void (...) @llvm.fake.use(ptr %selected)
  ret void
}

define goabiinternal void @hoisted_aggregate_pointer_alloca() gc "goallc" {
entry:
  %slot = alloca [2 x ptr], align 8
  %slice = insertvalue { ptr, i64, i64 } poison, ptr %slot, 0
  %slice.len = insertvalue { ptr, i64, i64 } %slice, i64 2, 1
  %slice.cap = insertvalue { ptr, i64, i64 } %slice.len, i64 2, 2
  call goabiinternal void @safepoint()
  call void @llvm.lifetime.start.p0(i64 16, ptr %slot)
  store ptr null, ptr %slot, align 8
  %second = getelementptr inbounds ptr, ptr %slot, i64 1
  store ptr null, ptr %second, align 8
  call goabiinternal void @observe_slice({ ptr, i64, i64 } %slice.cap)
  call void @llvm.lifetime.end.p0(i64 16, ptr %slot)
  call goabiinternal void @safepoint()
  ret void
}
