target triple = "x86_64-unknown-linux-goobj"

%packet = type { i64, ptr, [24 x i8], ptr }
%large = type { ptr, [304 x i8], ptr }

declare goabiinternal void @consume_packet(ptr byval(%packet) align 8)
declare goabiinternal void @consume_large(ptr byval(%large) align 8)
declare goabiinternal void @panic() noreturn
declare goabiinternal void @leaf() "gc-leaf-function"
declare void @llvm.memset.p0.i64(ptr, i8, i64, i1)

; No complete zero fill, a dynamic partial fill, a partial scalar overwrite,
; two pointer fields, and an interface-style nil edge exercise Go IR preparation.
; IR-LABEL: define goabiinternal void @generic_nil(
; IR-NOT: store
; IR-NOT: @llvm.memset
; IR: br i1 %nil
; IR: call:
; IR: store i64 %word
; IR: store i32 %upper
; IR: store ptr %first
; IR: store <16 x i8>
; IR: store <8 x i8>
; IR: store ptr %last
; IR: elementtype(void (ptr)) @consume_packet
; IR-NOT: "deopt"(
; IR: ret void
; MIR-LABEL: name: generic_nil
; MIR: stack: []
; MIR: STATEPOINT {{.*}}@consume_packet
; MIR-NOT: %stack.
; MIR: RET
define goabiinternal void @generic_nil(i1 %nil, i64 %word, i32 %upper,
                                       i8 %byte, ptr %first, ptr %last) gc "goallc" {
entry:
  %slot = alloca %packet, align 8
  store i64 %word, ptr %slot, align 8
  %p4 = getelementptr i8, ptr %slot, i64 4
  store i32 %upper, ptr %p4, align 4
  %p8 = getelementptr i8, ptr %slot, i64 8
  store ptr %first, ptr %p8, align 8
  %p16 = getelementptr i8, ptr %slot, i64 16
  call void @llvm.memset.p0.i64(ptr align 8 %p16, i8 %byte, i64 24, i1 false)
  %p40 = getelementptr i8, ptr %slot, i64 40
  store ptr %last, ptr %p40, align 8
  br i1 %nil, label %bad, label %call
call:
  call goabiinternal void @consume_packet(ptr byval(%packet) align 8 %slot)
  ret void
bad:
  call goabiinternal void @panic()
  unreachable
}

; The loaded pointer snapshot retains its SSA identity through preparation.
; IR-LABEL: define goabiinternal void @pointer_snapshot(
; IR: %snapshot = load ptr, ptr %source
; IR: store ptr %replacement, ptr %source
; IR: store ptr %snapshot
; IR: elementtype(void (ptr)) @consume_packet
; MIR-LABEL: name: pointer_snapshot
; MIR: stack: []
; MIR: STATEPOINT {{.*}}@consume_packet
; MIR-NOT: %stack.
; MIR: RET
define goabiinternal void @pointer_snapshot(ptr %source, ptr %replacement) gc "goallc" {
entry:
  %slot = alloca %packet, align 8
  %snapshot = load ptr, ptr %source
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 48, i1 false)
  %p8 = getelementptr i8, ptr %slot, i64 8
  store ptr %snapshot, ptr %p8, align 8
  store ptr %replacement, ptr %source
  call goabiinternal void @consume_packet(ptr byval(%packet) align 8 %slot)
  ret void
}

; A dynamic 304-byte data fill between two typed pointers demonstrates that
; neither full-zero initialization nor a 256-byte object limit is required.
; IR-LABEL: define goabiinternal void @large_typed(
; IR-NOT: @llvm.memset
; IR: store ptr %first
; IR: store <16 x i8>
; IR: store ptr %last
; IR: elementtype(void (ptr)) @consume_large
; MIR-LABEL: name: large_typed
; MIR: stack: []
; MIR: STATEPOINT {{.*}}@consume_large
; MIR-NOT: %stack.
; MIR: RET
define goabiinternal void @large_typed(i8 %byte, ptr %first, ptr %last) gc "goallc" {
entry:
  %slot = alloca %large, align 8
  store ptr %first, ptr %slot, align 8
  %data = getelementptr i8, ptr %slot, i64 8
  call void @llvm.memset.p0.i64(ptr align 8 %data, i8 %byte, i64 304, i1 false)
  %p312 = getelementptr i8, ptr %slot, i64 312
  store ptr %last, ptr %p312, align 8
  call goabiinternal void @consume_large(ptr byval(%large) align 8 %slot)
  ret void
}

; Even a GC-leaf call on only one path can reuse outgoing argument storage.
; IR-LABEL: define goabiinternal void @call_in_diamond(
; IR: call void @llvm.memset
; IR: store ptr %first
; IR: br i1 %choose
; IR: call goabiinternal void @leaf
; IR: elementtype(void (ptr)) @consume_packet
; MIR-LABEL: name: call_in_diamond
; MIR: stack:
; MIR: name: slot
; MIR: STATEPOINT {{.*}}@consume_packet
define goabiinternal void @call_in_diamond(i1 %choose, ptr %first) gc "goallc" {
entry:
  %slot = alloca %packet, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 48, i1 false)
  %p8 = getelementptr i8, ptr %slot, i64 8
  store ptr %first, ptr %p8, align 8
  br i1 %choose, label %side, label %call
side:
  call goabiinternal void @leaf()
  br label %call
call:
  call goabiinternal void @consume_packet(ptr byval(%packet) align 8 %slot)
  ret void
}

; Conditional initialization still requires a memory merge and is not sunk.
; IR-LABEL: define goabiinternal void @conditional_pointer(
; IR: call void @llvm.memset
; IR: br i1 %choose
; IR: store ptr %first
; IR: elementtype(void (ptr)) @consume_packet
; MIR-LABEL: name: conditional_pointer
; MIR: stack:
; MIR: name: slot
; MIR: STATEPOINT {{.*}}@consume_packet
define goabiinternal void @conditional_pointer(i1 %choose, ptr %first) gc "goallc" {
entry:
  %slot = alloca %packet, align 8
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 48, i1 false)
  br i1 %choose, label %write, label %call
write:
  %p8 = getelementptr i8, ptr %slot, i64 8
  store ptr %first, ptr %p8, align 8
  br label %call
call:
  call goabiinternal void @consume_packet(ptr byval(%packet) align 8 %slot)
  ret void
}
