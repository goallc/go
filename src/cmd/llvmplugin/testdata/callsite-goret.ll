target triple = "x86_64-unknown-linux-goobj"

%slice = type { ptr, i64, i64 }
%large_result = type { ptr, i64, i64, i64, i64, i64, i64, i64, i64 }

; OBJVIEW-LABEL: TEXT call_slice_abi0(SB)
; OBJVIEW-NOT: FUNCDATA_StackObjects
; OBJVIEW: R_CALL{{(ARM64)?}}:copySlice
; OBJVIEW-NEXT: {{.*}}ordinary safepoint{{.*}}LocalsPointerMaps={{0+}}
; OBJVIEW-NOT: FUNCDATA_StackObjects
;
; OBJVIEW-LABEL: TEXT live_value_call_slice_abi0(SB)
; OBJVIEW: R_CALL{{(ARM64)?}}:copySlice
; OBJVIEW-NEXT: {{.*}}ordinary safepoint{{.*}}LocalsPointerMaps={{0+}}
; OBJVIEW: R_CALL{{(ARM64)?}}:safepoint
; OBJVIEW-NEXT: {{.*}}ordinary safepoint{{.*}}LocalsPointerMaps={{[01]*1[01]*}}
;
; OBJVIEW-LABEL: TEXT observed_call_slice_abi0(SB)
; OBJVIEW: FUNCDATA_StackObjects independent object[0]
; OBJVIEW: R_CALL{{(ARM64)?}}:copySlice
; OBJVIEW-NEXT: {{.*}}ordinary safepoint{{.*}}LocalsPointerMaps={{0+}}
; OBJVIEW: R_CALL{{(ARM64)?}}:safepoint
; OBJVIEW-NEXT: {{.*}}ordinary safepoint{{.*}}LocalsPointerMaps={{[01]*1[01]*}}

; IR-LABEL: define goabiinternal %slice @call_slice_abi0(
; The ABI0 call defines the complete result carrier. Its pre-call contents are
; not a caller root and require neither zeroing nor pointer-map metadata.
; IR: %argument = alloca %slice, align 8
; IR: %result = alloca %slice, align 8
; IR-NOT: llvm.stackcoloring.no_merge
; IR-NOT: llvm.memset.inline
; IR: call goabi0 token {{.*}} @llvm.experimental.gc.statepoint
; IR-SAME: ptr byval(%slice) align 8 %argument
; IR-SAME: ptr goret(%slice) align 8 "goretindex"="0" %result
; IR-SAME: i32 0, i32 0)
; IR-NOT: "deopt"(
; IR-NOT: "gc-live"(

; MIR-LABEL: name: call_slice_abi0
; The slice argument is forwarded into the outgoing ABI0 frame. The result is
; used only by bounded scalar loads and does not cross another statepoint, so
; target call lowering reads those fields directly from the ABI0 result area.
; MIR-NOT: name: argument
; MIR-NOT: name: result
; MIR: stack: []
; MIR: STATEPOINT {{.*}}@"copySlice<ABI0>"
; MIR-COUNT-3: {{(MOV64rm|LDRXui)}} {{.*}} :: (load (s64) from stack
; MIR: ADJCALLSTACKUP
; MIR-NOT: %stack.{{[0-9]+}}.result
; MIR: RET

declare goabi0 void @"copySlice<ABI0>"(
    ptr byval(%slice) align 8,
    ptr goret(%slice) align 8 "goretindex"="0")

define goabiinternal %slice @call_slice_abi0(%slice %value) #0 gc "goallc" {
entry:
  %argument = alloca %slice, align 8
  %result = alloca %slice, align 8
  %base = extractvalue %slice %value, 0
  store ptr %base, ptr %argument, align 8
  %len.address = getelementptr inbounds i8, ptr %argument, i64 8
  %len = extractvalue %slice %value, 1
  store i64 %len, ptr %len.address, align 8
  %cap.address = getelementptr inbounds i8, ptr %argument, i64 16
  %cap = extractvalue %slice %value, 2
  store i64 %cap, ptr %cap.address, align 8
  call void @llvm.lifetime.start.p0(ptr %result)
  call goabi0 void @"copySlice<ABI0>"(
      ptr byval(%slice) align 8 %argument,
      ptr goret(%slice) align 8 "goretindex"="0" %result)
  %result.base = load ptr, ptr %result, align 8
  %result.len.address = getelementptr inbounds i8, ptr %result, i64 8
  %result.len = load i64, ptr %result.len.address, align 8
  %result.cap.address = getelementptr inbounds i8, ptr %result, i64 16
  %result.cap = load i64, ptr %result.cap.address, align 8
  %ret.base = insertvalue %slice poison, ptr %result.base, 0
  %ret.len = insertvalue %slice %ret.base, i64 %result.len, 1
  %ret.cap = insertvalue %slice %ret.len, i64 %result.cap, 2
  ret %slice %ret.cap
}

; IR-LABEL: define goabiinternal void @forwarded_incoming_slice_indirect(
; An indirect ABI0 target prevents the frontend carrier from using the narrow
; direct-call elision. Argument-copy elision can still map this private slice
; carrier to the incoming value's fixed register home. CodeGen should forward
; its SSA pieces to the ABI0 outgoing area without a fixed-home store/reload
; chain, while retaining the home for the independent morestack path.
; IR: %forward.argument = alloca %slice, align 8
; IR: call goabi0 token {{.*}} @llvm.experimental.gc.statepoint
; IR-SAME: ptr elementtype(void (ptr)) %callee
; IR-SAME: ptr byval(%slice) align 8 %forward.argument
;
; MIR-LABEL: name: forwarded_incoming_slice_indirect
; MIR: fixedStack:
; MIR: ADJCALLSTACKDOWN
; MIR-NOT: {{(MOV|STR|LDR)}}{{.*}}%fixed-stack
; MIR: STATEPOINT
; MIR: ADJCALLSTACKUP
define goabiinternal void @forwarded_incoming_slice_indirect(
    %slice %value, ptr %callee) #0 gc "goallc" {
entry:
  %forward.argument = alloca %slice, align 8
  store %slice %value, ptr %forward.argument, align 8
  call goabi0 void %callee(ptr byval(%slice) align 8 %forward.argument)
  ret void
}

; IR-LABEL: define goabiinternal i64 @indirect_slice_loop(
; The carrier is reused by an indirect ABI0 call in a loop and read after the
; following statepoint. It must therefore remain in both gc-live sets rather
; than being treated as a disposable private carrier.
; IR: %loop.argument = alloca %slice, align 8
; IR: call goabi0 token {{.*}} @llvm.experimental.gc.statepoint
; IR-SAME: ptr elementtype(void (ptr)) %callee
; IR-SAME: ptr byval(%slice) align 8 %loop.argument
; IR-SAME: "gc-live"({{.*}}ptr %loop.argument{{.*}})
; IR: call goabiinternal token {{.*}} @llvm.experimental.gc.statepoint
; IR-SAME: ptr elementtype(void ()) @safepoint
; IR-SAME: "gc-live"({{.*}}ptr %loop.argument{{.*}})
;
; MIR-LABEL: name: indirect_slice_loop
; MIR: bb.1.loop:
; MIR: ADJCALLSTACKDOWN
; MIR: STATEPOINT
; MIR: ADJCALLSTACKUP
; MIR: STATEPOINT
;
; X86 reserves the maximum outgoing area in the fixed frame. Do not pin the
; frame size, registers, or the complete instruction sequence; only ensure the
; loop itself does not grow and shrink the call frame or use PUSH arguments.
; ASM-X86-LABEL: indirect_slice_loop:
; ASM-X86: subq
; ASM-X86: [[LOOP:.LBB[0-9]+_[0-9]+]]:
; ASM-X86-NOT: subq
; ASM-X86-NOT: pushq
; ASM-X86: callq
; ASM-X86-NOT: addq
; ASM-X86: callq
define goabiinternal i64 @indirect_slice_loop(
    ptr %callee, %slice %value, i64 %iterations) #0 gc "goallc" {
entry:
  %loop.argument = alloca %slice, align 8
  br label %loop

loop:
  %index = phi i64 [ 0, %entry ], [ %next, %loop.cont ]
  store %slice %value, ptr %loop.argument, align 8
  call goabi0 void %callee(ptr byval(%slice) align 8 %loop.argument)
  call goabiinternal void @safepoint()
  %length.address = getelementptr inbounds i8, ptr %loop.argument, i64 8
  %length = load i64, ptr %length.address, align 8
  br label %loop.cont

loop.cont:
  %next = add nuw i64 %index, 1
  %more = icmp ult i64 %next, %iterations
  br i1 %more, label %loop, label %exit

exit:
  %answer = add i64 %next, %length
  ret i64 %answer
}

; IR-LABEL: define goabiinternal i64 @prior_safepoint_call_slice_abi0()
; A prior safepoint keeps both future call-carrier addresses in gc-live so
; stack growth cannot leave a cached address live in its continuation.
; IR: %prior.argument = alloca %slice, align 8
; IR: %prior.result = alloca %slice, align 8
; IR: call goabiinternal token {{.*}} @llvm.experimental.gc.statepoint
; IR-SAME: ptr elementtype(void ()) @safepoint
; IR-SAME: "gc-live"({{(ptr %prior.result, ptr %prior.argument|ptr %prior.argument, ptr %prior.result)}})
; IR: call goabi0 token {{.*}} @llvm.experimental.gc.statepoint
; IR-SAME: ptr byval(%slice) align 8 %prior.argument
; IR-SAME: ptr goret(%slice) align 8 "goretindex"="0" %prior.result
;
; MIR-LABEL: name: prior_safepoint_call_slice_abi0
; Direct gc-live uses are address-only for these proven call carriers. Once
; call lowering forwards their contents to and from the physical ABI0 area,
; neither local FrameIndex remains. This is also the regression shape in
; floats.LogSumExp, where earlier ABI0 calls made later carriers gc-live.
; MIR: stack: []
; MIR: STATEPOINT {{.*}}@safepoint
; MIR-COUNT-3: {{(MOV64m(i32|r)|STRXui)}} {{.*}} :: (store (s64) into stack
; MIR: STATEPOINT {{.*}}@"copySlice<ABI0>"
; MIR-COUNT-3: {{(MOV64rm|LDRXui)}} {{.*}} :: (load (s64) from stack
; MIR-NOT: %stack.
; MIR: RET
define goabiinternal i64 @prior_safepoint_call_slice_abi0() #0 gc "goallc" {
entry:
  %prior.argument = alloca %slice, align 8
  %prior.result = alloca %slice, align 8
  call goabiinternal void @safepoint()
  store ptr null, ptr %prior.argument, align 8
  %prior.len.address = getelementptr inbounds i8, ptr %prior.argument, i64 8
  store i64 4, ptr %prior.len.address, align 8
  %prior.cap.address = getelementptr inbounds i8, ptr %prior.argument, i64 16
  store i64 8, ptr %prior.cap.address, align 8
  call void @llvm.lifetime.start.p0(ptr %prior.result)
  call goabi0 void @"copySlice<ABI0>"(
      ptr byval(%slice) align 8 %prior.argument,
      ptr goret(%slice) align 8 "goretindex"="0" %prior.result)
  %prior.result.base = load ptr, ptr %prior.result, align 8
  %prior.result.base.bits = ptrtoint ptr %prior.result.base to i64
  %prior.result.len.address =
      getelementptr inbounds i8, ptr %prior.result, i64 8
  %prior.result.len = load i64, ptr %prior.result.len.address, align 8
  %prior.result.cap.address =
      getelementptr inbounds i8, ptr %prior.result, i64 16
  %prior.result.cap = load i64, ptr %prior.result.cap.address, align 8
  %prior.sum.len = add i64 %prior.result.base.bits, %prior.result.len
  %prior.sum.cap = add i64 %prior.sum.len, %prior.result.cap
  ret i64 %prior.sum.cap
}

; IR-LABEL: define goabiinternal %slice @live_value_call_slice_abi0(
; The carrier address is dead at its defining call, but a pointer loaded from
; the returned slice is ordinary SSA data and remains explicitly tracked when
; it crosses a later statepoint.
; IR: %live.result = alloca %slice, align 8
; IR: call goabi0 token {{.*}} @llvm.experimental.gc.statepoint
; IR-SAME: ptr goret(%slice) align 8 "goretindex"="0" %live.result
; IR-SAME: i32 0, i32 0)
; IR: %live.result.base = load ptr, ptr %live.result
; IR: call goabiinternal token {{.*}} @llvm.experimental.gc.statepoint
; IR-SAME: ptr elementtype(void ()) @safepoint
; IR-SAME: "gc-live"(ptr %live.result.base)
; IR: call coldcc ptr @llvm.experimental.gc.relocate

; MIR-LABEL: name: live_value_call_slice_abi0
; A pointer field which crosses the following statepoint must remain backed by
; the ordinary carrier so statepoint liveness and gc.relocate stay authoritative.
; MIR: name: live.result
; MIR: STATEPOINT {{.*}}@"copySlice<ABI0>"
; MIR: {{(MOV64mr|STR(Q|X)ui)}} {{.*}}%stack.{{[0-9]+}}.live.result
; MIR: STATEPOINT {{.*}}@safepoint

declare goabiinternal void @safepoint()

define goabiinternal %slice @live_value_call_slice_abi0(%slice %value) #0 gc "goallc" {
entry:
  %live.argument = alloca %slice, align 8
  %live.result = alloca %slice, align 8
  %base = extractvalue %slice %value, 0
  store ptr %base, ptr %live.argument, align 8
  %len.address = getelementptr inbounds i8, ptr %live.argument, i64 8
  %len = extractvalue %slice %value, 1
  store i64 %len, ptr %len.address, align 8
  %cap.address = getelementptr inbounds i8, ptr %live.argument, i64 16
  %cap = extractvalue %slice %value, 2
  store i64 %cap, ptr %cap.address, align 8
  call void @llvm.lifetime.start.p0(ptr %live.result)
  call goabi0 void @"copySlice<ABI0>"(
      ptr byval(%slice) align 8 %live.argument,
      ptr goret(%slice) align 8 "goretindex"="0" %live.result)
  %live.result.base = load ptr, ptr %live.result, align 8
  %live.result.len.address = getelementptr inbounds i8, ptr %live.result, i64 8
  %live.result.len = load i64, ptr %live.result.len.address, align 8
  %live.result.cap.address = getelementptr inbounds i8, ptr %live.result, i64 16
  %live.result.cap = load i64, ptr %live.result.cap.address, align 8
  call goabiinternal void @safepoint()
  %ret.base = insertvalue %slice poison, ptr %live.result.base, 0
  %ret.len = insertvalue %slice %ret.base, i64 %live.result.len, 1
  %ret.cap = insertvalue %slice %ret.len, i64 %live.result.cap, 2
  ret %slice %ret.cap
}

; IR-LABEL: define goabiinternal %slice @observed_call_slice_abi0(
; An independently observable result address is not a pure goret carrier.
; Preserve the fixed-frame root and its rematerialization across both the
; defining call (contents not live yet) and a later safepoint (contents live).
; IR: %observed.result = alloca %slice, align 8, !llvm.stackcoloring.no_merge
; IR: call goabi0 token {{.*}} @llvm.experimental.gc.statepoint
; IR-SAME: ptr goret(%slice) align 8 "goretindex"="0" %observed.result
; IR-SAME: "deopt"({{.*}}ptr %observed.result, i64 0, i64 24, i64 8, i64 8, i64 0,
; IR-SAME: "gc-live"(ptr %observed.result)

; MIR-LABEL: name: observed_call_slice_abi0
; MIR: name: observed.result
; MIR: STATEPOINT {{.*}}@"copySlice<ABI0>"
; MIR: {{(MOV64mr|STR(Q|X)ui)}} {{.*}}%stack.{{[0-9]+}}.observed.result
; MIR: STATEPOINT {{.*}}@safepoint
; IR: call goabiinternal token {{.*}} @llvm.experimental.gc.statepoint
; IR-SAME: ptr elementtype(void ()) @safepoint
; IR-SAME: "deopt"({{.*}}ptr %observed.result, i64 0, i64 24, i64 8, i64 8, i64 1,
; IR-SAME: "gc-live"(ptr %observed.result)

declare goabiinternal void @observe(ptr)

define goabiinternal %slice @observed_call_slice_abi0(%slice %value) #0 gc "goallc" {
entry:
  %observed.argument = alloca %slice, align 8
  %observed.result = alloca %slice, align 8
  %base = extractvalue %slice %value, 0
  store ptr %base, ptr %observed.argument, align 8
  %len.address = getelementptr inbounds i8, ptr %observed.argument, i64 8
  %len = extractvalue %slice %value, 1
  store i64 %len, ptr %len.address, align 8
  %cap.address = getelementptr inbounds i8, ptr %observed.argument, i64 16
  %cap = extractvalue %slice %value, 2
  store i64 %cap, ptr %cap.address, align 8
  call void @llvm.lifetime.start.p0(ptr %observed.result)
  call goabi0 void @"copySlice<ABI0>"(
      ptr byval(%slice) align 8 %observed.argument,
      ptr goret(%slice) align 8 "goretindex"="0" %observed.result)
  call goabiinternal void @safepoint()
  call goabiinternal void @observe(ptr %observed.result)
  %result.base = load ptr, ptr %observed.result, align 8
  %result.len.address = getelementptr inbounds i8, ptr %observed.result, i64 8
  %result.len = load i64, ptr %result.len.address, align 8
  %result.cap.address = getelementptr inbounds i8, ptr %observed.result, i64 16
  %result.cap = load i64, ptr %result.cap.address, align 8
  %ret.base = insertvalue %slice poison, ptr %result.base, 0
  %ret.len = insertvalue %slice %ret.base, i64 %result.len, 1
  %ret.cap = insertvalue %slice %ret.len, i64 %result.cap, 2
  ret %slice %ret.cap
}

; IR-LABEL: define goabiinternal i64 @bounded_large_goret()
; More than eight scalar projections deliberately stays on the memory path,
; keeping large aggregate results out of SelectionDAG SSA lowering.
; IR: %large.result = alloca %large_result, align 8
; IR: call goabi0 token {{.*}} @llvm.experimental.gc.statepoint
; IR-SAME: ptr goret(%large_result) align 8 "goretindex"="0" %large.result
;
; MIR-LABEL: name: bounded_large_goret
; MIR: name: large.result
; MIR: STATEPOINT {{.*}}@"largeResult<ABI0>"
; MIR: ADJCALLSTACKUP
; MIR: {{(MOV64rm|LDRXui)}} %stack.{{[0-9]+}}.large.result
; MIR: RET

declare goabi0 void @"largeResult<ABI0>"(
    ptr goret(%large_result) align 8 "goretindex"="0")

define goabiinternal i64 @bounded_large_goret() #0 gc "goallc" {
entry:
  %large.result = alloca %large_result, align 8
  call void @llvm.lifetime.start.p0(ptr %large.result)
  call goabi0 void @"largeResult<ABI0>"(
      ptr goret(%large_result) align 8 "goretindex"="0" %large.result)
  %v0.ptr = load ptr, ptr %large.result, align 8
  %v0 = ptrtoint ptr %v0.ptr to i64
  %p1 = getelementptr inbounds i8, ptr %large.result, i64 8
  %v1 = load i64, ptr %p1, align 8
  %s1 = add i64 %v0, %v1
  %p2 = getelementptr inbounds i8, ptr %large.result, i64 16
  %v2 = load i64, ptr %p2, align 8
  %s2 = add i64 %s1, %v2
  %p3 = getelementptr inbounds i8, ptr %large.result, i64 24
  %v3 = load i64, ptr %p3, align 8
  %s3 = add i64 %s2, %v3
  %p4 = getelementptr inbounds i8, ptr %large.result, i64 32
  %v4 = load i64, ptr %p4, align 8
  %s4 = add i64 %s3, %v4
  %p5 = getelementptr inbounds i8, ptr %large.result, i64 40
  %v5 = load i64, ptr %p5, align 8
  %s5 = add i64 %s4, %v5
  %p6 = getelementptr inbounds i8, ptr %large.result, i64 48
  %v6 = load i64, ptr %p6, align 8
  %s6 = add i64 %s5, %v6
  %p7 = getelementptr inbounds i8, ptr %large.result, i64 56
  %v7 = load i64, ptr %p7, align 8
  %s7 = add i64 %s6, %v7
  %p8 = getelementptr inbounds i8, ptr %large.result, i64 64
  %v8 = load i64, ptr %p8, align 8
  %s8 = add i64 %s7, %v8
  ret i64 %s8
}

declare void @llvm.lifetime.start.p0(ptr nocapture) #1

attributes #0 = { "frame-pointer"="non-leaf" }
attributes #1 = { mustprogress nocallback nofree nosync nounwind willreturn memory(argmem: readwrite) }
