target triple = "x86_64-unknown-linux-goobj"

%entry = type { i8, [7 x i8], ptr, [120 x i8] }
@escape = external global ptr

declare void @llvm.memset.p0.i64(ptr, i8, i64, i1)
declare void @llvm.lifetime.start.p0(ptr)
declare goabiinternal void @consume(ptr byval(%entry) align 8)
declare goabiinternal void @safepoint()
declare goabiinternal void @leaf() "gc-leaf-function"
declare goabiinternal void @panic() noreturn

; The stores and their typed pointer value move into the call block before
; GC liveness. The existing byval-carrier proof can then remove the source.
; IR-LABEL: define goabiinternal void @nil_edge(
; IR: entry:
; IR-NOT: @llvm.memset
; IR-NOT: store
; IR: br i1 %nil, label %bad, label %call
; IR: call:
; IR: call void @llvm.lifetime.start.p0(ptr %slot)
; IR: store i8 0
; IR: store <2 x i8> zeroinitializer
; IR: store <4 x i8> zeroinitializer
; IR: store <16 x i8> zeroinitializer
; IR: store <8 x i8> zeroinitializer
; IR: store i8 %level
; IR: store ptr %pointer
; IR: elementtype(void (ptr)) @consume
; IR-NOT: "deopt"(
; IR: ret void
; O0-LABEL: define goabiinternal void @nil_edge(
; O0: call void @llvm.memset
; O0: store i8 %level
; O0: br i1 %nil
; MIR-LABEL: name: nil_edge
; MIR: stack: []
; MIR: STATEPOINT {{.*}}@consume
; MIR-NOT: %stack.
; MIR: RET

define goabiinternal void @nil_edge(i1 %nil, i8 %level, ptr %pointer) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.lifetime.start.p0(ptr %slot)
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store i8 %level, ptr %slot, align 8
  %field = getelementptr i8, ptr %slot, i64 8
  store ptr %pointer, ptr %field, align 8
  br i1 %nil, label %bad, label %call
call:
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
bad:
  call goabiinternal void @panic()
  unreachable
}

; IR-LABEL: define goabiinternal void @same_block(
; IR-NOT: @llvm.memset
; IR: store i8 0
; IR: store <2 x i8> zeroinitializer
; IR: store <4 x i8> zeroinitializer
; IR: store <16 x i8> zeroinitializer
; IR: store <8 x i8> zeroinitializer
; IR: store i8 %level
; IR: store ptr %pointer
; IR: elementtype(void (ptr)) @consume
; IR-NOT: "deopt"(
; IR: ret void
define goabiinternal void @same_block(i8 %level, ptr %pointer, ptr %out, i1 %choose) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store i8 %level, ptr %slot, align 8
  %field = getelementptr i8, ptr %slot, i64 8
  store ptr %pointer, ptr %field, align 8
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; IR-LABEL: define goabiinternal void @overwritten_store(
; IR-NOT: @llvm.memset
; IR: store i8 0
; IR: store <2 x i8> zeroinitializer
; IR: store <4 x i8> zeroinitializer
; IR: store <16 x i8> zeroinitializer
; IR: store <8 x i8> zeroinitializer
; IR: store i8 %level
; IR: store ptr %pointer
; IR: elementtype(void (ptr)) @consume
; IR-NOT: "deopt"(
; IR: ret void
define goabiinternal void @overwritten_store(i8 %level, ptr %pointer, ptr %out, i1 %choose) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store i8 %level, ptr %slot, align 8
  %field = getelementptr i8, ptr %slot, i64 8
  store i64 123, ptr %field, align 8
  store ptr %pointer, ptr %field, align 8
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; IR-LABEL: define goabiinternal void @intervening_safepoint(
; IR: call void @llvm.memset
; IR: store i8 %level
; IR: elementtype(void (ptr)) @consume
define goabiinternal void @intervening_safepoint(i8 %level, ptr %pointer, ptr %out, i1 %choose) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store i8 %level, ptr %slot, align 8
  %field = getelementptr i8, ptr %slot, i64 8
  store ptr %pointer, ptr %field, align 8
  call goabiinternal void @safepoint()
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; IR-LABEL: define goabiinternal void @intervening_leaf_call(
; IR: call void @llvm.memset
; IR: store i8 %level
; IR: elementtype(void (ptr)) @consume
define goabiinternal void @intervening_leaf_call(i8 %level, ptr %pointer, ptr %out, i1 %choose) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store i8 %level, ptr %slot, align 8
  %field = getelementptr i8, ptr %slot, i64 8
  store ptr %pointer, ptr %field, align 8
  call goabiinternal void @leaf()
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; IR-LABEL: define goabiinternal void @partial_overwrite(
; IR-NOT: @llvm.memset
; IR: store i8 %level
; IR: elementtype(void (ptr)) @consume
; MIR-LABEL: name: partial_overwrite
; MIR: stack: []
; MIR: STATEPOINT
; MIR-NOT: %stack.
; MIR: RET

define goabiinternal void @partial_overwrite(i8 %level, ptr %pointer, ptr %out, i1 %choose) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store i8 %level, ptr %slot, align 8
  %field = getelementptr i8, ptr %slot, i64 8
  store ptr %pointer, ptr %field, align 8
  %word = getelementptr i8, ptr %slot, i64 16
  store i64 123, ptr %word, align 8
  %part = getelementptr i8, ptr %slot, i64 20
  store i32 1, ptr %part, align 4
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; IR-LABEL: define goabiinternal void @volatile_clear(
; IR: call void @llvm.memset
; IR: store i8 %level
; IR: elementtype(void (ptr)) @consume
define goabiinternal void @volatile_clear(i8 %level, ptr %pointer, ptr %out, i1 %choose) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 true)
  store i8 %level, ptr %slot, align 8
  %field = getelementptr i8, ptr %slot, i64 8
  store ptr %pointer, ptr %field, align 8
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; IR-LABEL: define goabiinternal void @nonzero_clear(
; IR-NOT: @llvm.memset
; IR: store i8 %level
; IR: elementtype(void (ptr)) @consume
; MIR-LABEL: name: nonzero_clear
; MIR: stack: []
; MIR: STATEPOINT
; MIR-NOT: %stack.
; MIR: RET

define goabiinternal void @nonzero_clear(i8 %level, ptr %pointer, ptr %out, i1 %choose) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 1, i64 136, i1 false)
  store i8 %level, ptr %slot, align 8
  %field = getelementptr i8, ptr %slot, i64 8
  store ptr %pointer, ptr %field, align 8
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; IR-LABEL: define goabiinternal void @partial_clear(
; IR-NOT: @llvm.memset
; IR: store i8 %level
; IR: elementtype(void (ptr)) @consume
; MIR-LABEL: name: partial_clear
; MIR: stack: []
; MIR: STATEPOINT
; MIR-NOT: %stack.
; MIR: RET

define goabiinternal void @partial_clear(i8 %level, ptr %pointer, ptr %out, i1 %choose) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 128, i1 false)
  store i8 %level, ptr %slot, align 8
  %field = getelementptr i8, ptr %slot, i64 8
  store ptr %pointer, ptr %field, align 8
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; IR-LABEL: define goabiinternal void @escaping_address(
; IR: call void @llvm.memset
; IR: store i8 %level
; IR: elementtype(void (ptr)) @consume
define goabiinternal void @escaping_address(i8 %level, ptr %pointer, ptr %out, i1 %choose) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store i8 %level, ptr %slot, align 8
  %field = getelementptr i8, ptr %slot, i64 8
  store ptr %pointer, ptr %field, align 8
  store ptr %slot, ptr @escape, align 8
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; IR-LABEL: define goabiinternal void @two_calls(
; IR: call void @llvm.memset
; IR: store i8 %level
; IR: elementtype(void (ptr)) @consume
define goabiinternal void @two_calls(i8 %level, ptr %pointer, ptr %out, i1 %choose) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store i8 %level, ptr %slot, align 8
  %field = getelementptr i8, ptr %slot, i64 8
  store ptr %pointer, ptr %field, align 8
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; IR-LABEL: define goabiinternal void @volatile_field(
; IR: call void @llvm.memset
; IR: store i8 %level
; IR: elementtype(void (ptr)) @consume
define goabiinternal void @volatile_field(i8 %level, ptr %pointer, ptr %out, i1 %choose) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store i8 %level, ptr %slot, align 8
  %field = getelementptr i8, ptr %slot, i64 8
  store volatile ptr %pointer, ptr %field, align 8
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; IR-LABEL: define goabiinternal void @observed_source(
; IR: call void @llvm.memset
; IR: store i8 %level
; IR: elementtype(void (ptr)) @consume
define goabiinternal void @observed_source(i8 %level, ptr %pointer, ptr %out, i1 %choose) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store i8 %level, ptr %slot, align 8
  %field = getelementptr i8, ptr %slot, i64 8
  store ptr %pointer, ptr %field, align 8
  %loaded = load i8, ptr %slot, align 8
  store volatile i8 %loaded, ptr %out
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; IR-LABEL: define goabiinternal void @merge_edge(
; IR-NOT: @llvm.memset
; IR: store i8 %level
; IR: elementtype(void (ptr)) @consume
; MIR-LABEL: name: merge_edge
; MIR: stack: []
; MIR: STATEPOINT
; MIR-NOT: %stack.
; MIR: RET

define goabiinternal void @merge_edge(i8 %level, ptr %pointer, ptr %out, i1 %choose) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store i8 %level, ptr %slot, align 8
  %field = getelementptr i8, ptr %slot, i64 8
  store ptr %pointer, ptr %field, align 8
  br i1 %choose, label %left, label %right
left:
  br label %merge
right:
  br label %merge
merge:
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; A stronger local alignment and an ordinary C caller retain their original
; homes. Preparation must not silently relax the existing carrier ABI proof.
declare goabiinternal void @consume_bytes(ptr byval([136 x i8]) align 8)
; IR-LABEL: define goabiinternal void @overaligned(
; IR: call void @llvm.memset
; IR: store i8 %level
; IR: elementtype(void (ptr)) @consume_bytes
define goabiinternal void @overaligned(i8 %level) gc "goallc" {
entry:
  %slot = alloca [136 x i8], align 16
  call void @llvm.memset.p0.i64(ptr align 16 %slot, i8 0, i64 136, i1 false)
  store i8 %level, ptr %slot, align 16
  call goabiinternal void @consume_bytes(ptr byval([136 x i8]) align 8 %slot)
  ret void
}

; IR-LABEL: define void @c_caller(
; IR: call void @llvm.memset
; IR: store i8 %level
; IR: call goabiinternal void @consume
define void @c_caller(i8 %level) {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store i8 %level, ptr %slot, align 8
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; A 257-byte object is no longer rejected just because of its size.
; IR-LABEL: define goabiinternal void @large_object(
; IR-NOT: @llvm.memset
; IR: store i8 %level
; IR: elementtype(void (ptr)) @consume_large
declare goabiinternal void @consume_large(ptr byval([257 x i8]) align 8)
; MIR-LABEL: name: large_object
; MIR: stack: []
; MIR: STATEPOINT
; MIR-NOT: %stack.
; MIR: RET

define goabiinternal void @large_object(i8 %level) gc "goallc" {
entry:
  %slot = alloca [257 x i8], align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 257, i1 false)
  store i8 %level, ptr %slot, align 8
  call goabiinternal void @consume_large(ptr byval([257 x i8]) align 8 %slot)
  ret void
}

; Multiple lifetime intervals cannot be shortened to one call's initialization.
; IR-LABEL: define goabiinternal void @lifetime_end(
; IR: call void @llvm.memset
; IR: store i8 %level
; IR: elementtype(void (ptr)) @consume
; IR: call void @llvm.lifetime.end
declare void @llvm.lifetime.end.p0(ptr)
define goabiinternal void @lifetime_end(i8 %level) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.lifetime.start.p0(ptr %slot)
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store i8 %level, ptr %slot, align 8
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  call void @llvm.lifetime.end.p0(ptr %slot)
  ret void
}

; A loaded snapshot is retained in its original SSA value
; even when the source memory changes before the consuming call.
; IR-LABEL: define goabiinternal void @limit_and_snapshot(
; IR: %snapshot = load i8, ptr %source
; IR-NOT: @llvm.memset
; IR: store i8 99, ptr %source
; IR: store <16 x i8> zeroinitializer
; IR: store i8 %snapshot
; IR: elementtype(void (ptr)) @consume_limit
declare goabiinternal void @consume_limit(ptr byval([256 x i8]) align 8)
define goabiinternal void @limit_and_snapshot(ptr %source) gc "goallc" {
entry:
  %slot = alloca [256 x i8], align 8
  %snapshot = load i8, ptr %source
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 256, i1 false)
  store i8 %snapshot, ptr %slot, align 8
  store i8 99, ptr %source
  call goabiinternal void @consume_limit(ptr byval([256 x i8]) align 8 %slot)
  ret void
}

; Atomic stores and sub-byte stores cannot be rebuilt as ordinary byte stores.
; IR-LABEL: define goabiinternal void @atomic_field(
; IR: call void @llvm.memset
; IR: store atomic i8 %level
; IR: elementtype(void (ptr)) @consume
define goabiinternal void @atomic_field(i8 %level) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store atomic i8 %level, ptr %slot monotonic, align 8
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; IR-LABEL: define goabiinternal void @sub_byte_field(
; IR: call void @llvm.memset
; IR: store i1 %level
; IR: elementtype(void (ptr)) @consume
define goabiinternal void @sub_byte_field(i1 %level) gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store i1 %level, ptr %slot, align 8
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}

; Function-level optnone also disables preparation with optimized codegen.
; IR-LABEL: define goabiinternal void @no_optimization(
; IR: call void @llvm.memset
; IR: store i8 %level
; IR: elementtype(void (ptr)) @consume
define goabiinternal void @no_optimization(i8 %level) noinline optnone gc "goallc" {
entry:
  %slot = alloca %entry, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 136, i1 false)
  store i8 %level, ptr %slot, align 8
  call goabiinternal void @consume(ptr byval(%entry) align 8 %slot)
  ret void
}
