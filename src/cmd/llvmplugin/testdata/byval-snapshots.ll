target triple = "x86_64-unknown-linux-goobj"

%entry = type { [16 x i64], ptr }
%sparse = type [136 x i8]
declare goabiinternal void @consume(ptr byval(%entry) align 8)
declare goabiinternal void @consume_sparse(ptr byval(%sparse) align 8)
declare goabiinternal void @mutate()
declare goabiinternal void @leaf() "gc-leaf-function"
declare goabiinternal void @panic() noreturn
declare void @llvm.memset.p0.i64(ptr, i8, i64, i1)

; All 136 bytes are initialized without any memset, before an interface-style
; nil edge. The original SSA values must be written directly into outgoing
; storage on the call path, with no construction temporary.
; IR-LABEL: define goabiinternal void @full_literal(
; IR-NOT: store
; IR: br i1 %nil
; IR: call:
; IR-NOT: @llvm.memset
; IR: store i64 %word
; IR: store ptr %pointer
; IR: elementtype(void (ptr)) @consume
; MIR-LABEL: name: full_literal
; MIR: stack: []
; MIR: STATEPOINT {{.*}}@consume
; MIR-NOT: %stack.
; MIR: RET
define goabiinternal void @full_literal(i1 %nil, i64 %word, ptr %pointer) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  store i64 %word, ptr %slot, align 8
  %p1 = getelementptr i8, ptr %slot, i64 8
  store i64 %word, ptr %p1, align 8
  %p2 = getelementptr i8, ptr %slot, i64 16
  store i64 %word, ptr %p2, align 8
  %p3 = getelementptr i8, ptr %slot, i64 24
  store i64 %word, ptr %p3, align 8
  %p4 = getelementptr i8, ptr %slot, i64 32
  store i64 %word, ptr %p4, align 8
  %p5 = getelementptr i8, ptr %slot, i64 40
  store i64 %word, ptr %p5, align 8
  %p6 = getelementptr i8, ptr %slot, i64 48
  store i64 %word, ptr %p6, align 8
  %p7 = getelementptr i8, ptr %slot, i64 56
  store i64 %word, ptr %p7, align 8
  %p8 = getelementptr i8, ptr %slot, i64 64
  store i64 %word, ptr %p8, align 8
  %p9 = getelementptr i8, ptr %slot, i64 72
  store i64 %word, ptr %p9, align 8
  %p10 = getelementptr i8, ptr %slot, i64 80
  store i64 %word, ptr %p10, align 8
  %p11 = getelementptr i8, ptr %slot, i64 88
  store i64 %word, ptr %p11, align 8
  %p12 = getelementptr i8, ptr %slot, i64 96
  store i64 %word, ptr %p12, align 8
  %p13 = getelementptr i8, ptr %slot, i64 104
  store i64 %word, ptr %p13, align 8
  %p14 = getelementptr i8, ptr %slot, i64 112
  store i64 %word, ptr %p14, align 8
  %p15 = getelementptr i8, ptr %slot, i64 120
  store i64 %word, ptr %p15, align 8
  %last = getelementptr i8, ptr %slot, i64 128
  store ptr %pointer, ptr %last, align 8
  br i1 %nil, label %bad, label %call
bad:
  call goabiinternal void @panic()
  unreachable
call:
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; Scalar/pointer snapshots predate mutate, but constructing the byval object
; begins afterwards. Preserve the pointer through gc-live/gc.relocate and
; combine the 120-byte interior fill with the i64/ptr endpoint writes.
; IR-LABEL: define goabiinternal void @partial_after_mutate(
; IR: elementtype(void ()) @mutate
; IR-SAME: "gc-live"(ptr %pointer{{[,)]}}
; IR: %pointer.relocated = call coldcc ptr @llvm.experimental.gc.relocate
; IR-NOT: store
; IR: br i1 %nil
; IR: call:
; IR-NOT: @llvm.memset
; IR: store <16 x i8> zeroinitializer
; IR: store i64 %word
; IR: store ptr %pointer.relocated
; IR: elementtype(void (ptr)) @consume
; MIR-LABEL: name: partial_after_mutate
; MIR-NOT: size: 136
; MIR: STATEPOINT {{.*}}@mutate
; MIR: STATEPOINT {{.*}}@consume
; MIR: RET
define goabiinternal void @partial_after_mutate(i1 %nil, i64 %word, ptr %pointer) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call goabiinternal void @mutate()
  %middle = getelementptr i8, ptr %slot, i64 8
  call void @llvm.memset.p0.i64(ptr align 8 %middle, i8 0, i64 120, i1 false)
  store i64 %word, ptr %slot, align 8
  %last = getelementptr i8, ptr %slot, i64 128
  store ptr %pointer, ptr %last, align 8
  br i1 %nil, label %bad, label %call
bad:
  call goabiinternal void @panic()
  unreachable
call:
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; Building the object before mutate remains a conservative fallback. Keep
; its typed pointer in a GC-visible object across the intervening safepoint;
; do not pretend that the manual cross-call diagnostic is an automatic pass.
; IR-LABEL: define goabiinternal void @before_mutate(
; IR: call void @llvm.memset
; IR: store i64 %word
; IR: store ptr %pointer
; IR: elementtype(void ()) @mutate
; IR: elementtype(void (ptr)) @consume
; MIR-LABEL: name: before_mutate
; MIR: name: slot
; MIR-SAME: size: 136
; MIR: STATEPOINT {{.*}}@mutate
; MIR: STATEPOINT {{.*}}@consume
; MIR: RET
define goabiinternal void @before_mutate(i64 %word, ptr %pointer) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  %middle = getelementptr i8, ptr %slot, i64 8
  call void @llvm.memset.p0.i64(ptr align 8 %middle, i8 0, i64 120, i1 false)
  store i64 %word, ptr %slot, align 8
  %last = getelementptr i8, ptr %slot, i64 128
  store ptr %pointer, ptr %last, align 8
  call goabiinternal void @mutate()
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; A GC-leaf call is still an outgoing-area reuse barrier.
; IR-LABEL: define goabiinternal void @before_leaf(
; IR: call void @llvm.memset
; IR: store ptr %pointer
; IR: call goabiinternal void @leaf
; IR: elementtype(void (ptr)) @consume
; MIR-LABEL: name: before_leaf
; MIR: name: slot
; MIR-SAME: size: 136
; MIR: @leaf
; MIR: STATEPOINT {{.*}}@consume
; MIR: RET
define goabiinternal void @before_leaf(i64 %word, ptr %pointer) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store i64 %word, ptr %slot, align 8
  %last = getelementptr i8, ptr %slot, i64 128
  store ptr %pointer, ptr %last, align 8
  call goabiinternal void @leaf()
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; Unwritten bytes of this private scalar-only object were unspecified already.
; Forward only the two defined words; do not invent a zero fill for the hole.
; The runtime source test exercises fully initialized Go values separately.
; IR-LABEL: define goabiinternal void @sparse_words(
; IR-NOT: store
; IR: br i1 %nil
; IR: call:
; IR-NOT: @llvm.memset
; IR-NOT: zeroinitializer
; IR: store i64 %first
; IR-NOT: zeroinitializer
; IR: store i64 %last
; IR-NOT: zeroinitializer
; IR: elementtype(void (ptr)) @consume_sparse
; MIR-LABEL: name: sparse_words
; MIR: stack: []
; MIR: STATEPOINT {{.*}}@consume_sparse
; MIR-NOT: %stack.
; MIR: RET
define goabiinternal void @sparse_words(i1 %nil, i64 %first, i64 %last) gc "goallc" {
entry:
  %slot = alloca %sparse, align 8
  store i64 %first, ptr %slot, align 8
  %tail = getelementptr i8, ptr %slot, i64 128
  store i64 %last, ptr %tail, align 8
  br i1 %nil, label %bad, label %call
bad:
  call goabiinternal void @panic()
  unreachable
call:
  call goabiinternal void @consume_sparse(ptr byval(%sparse) align 8 %slot)
  ret void
}

; A defer home remains at its original lifetime even with a single byval use.
; IR-LABEL: define goabiinternal void @defer_home(
; IR: call void @llvm.memset
; IR: store ptr %pointer
; IR: br i1 %nil
; IR: elementtype(void (ptr)) @consume
; MIR-LABEL: name: defer_home
; MIR: name: slot
; MIR-SAME: size: 136
; MIR: STATEPOINT {{.*}}@consume
; MIR: RET
define goabiinternal void @defer_home(i1 %nil, ptr %pointer) gc "goallc" {
entry:
  %slot = alloca %entry, align 8, !goallc.defer_result !0
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  %last = getelementptr i8, ptr %slot, i64 128
  store ptr %pointer, ptr %last, align 8
  br i1 %nil, label %bad, label %call
bad:
  call goabiinternal void @panic()
  unreachable
call:
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

!0 = !{}
