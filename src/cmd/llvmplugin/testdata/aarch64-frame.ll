target triple = "aarch64-apple-darwin-goobj"

; OBJVIEW-LABEL: "name": "aarch64_pointer_and_code_live"
; OBJVIEW: "locals": [[#LOCALS:]]
; OBJVIEW: "kind": "pcsp"
; OBJVIEW: "value": [[#LOCALS+8]]
; OBJVIEW: "kind": "stack_map_index"
; OBJVIEW: "kind": "args_pointer_maps"
; OBJVIEW: "count": 2
; OBJVIEW-NEXT: "num_bits": 2
; OBJVIEW: "index": 0
; OBJVIEW: "set_bits": [
; OBJVIEW-NEXT: 1
; OBJVIEW: "index": 1
; OBJVIEW-NEXT: "set_bits": null
; OBJVIEW: "kind": "locals_pointer_maps"
; OBJVIEW: "count": 2
; OBJVIEW-NEXT: "num_bits": 2
; OBJVIEW: "index": 0
; OBJVIEW-NEXT: "set_bits": null
; OBJVIEW: "index": 1
; OBJVIEW: "set_bits": [
; OBJVIEW-NEXT: 1

; FRAME-TEXT-LABEL: TEXT aarch64_pointer_and_code_live(SB)
; FRAME-TEXT: PCDATA_StackMapIndex=-1
; FRAME-TEXT: R_CALLARM64:runtime.morestack_noctxt
; FRAME-TEXT-NEXT: {{.*}}stack-growth safepoint{{.*}}map[0]{{.*}}ArgsPointerMaps=01{{.*}}LocalsPointerMaps=00
; FRAME-TEXT: R_CALLIND{{.*}}PCDATA_StackMapIndex=1{{.*}}ArgsPointerMaps=00{{.*}}LocalsPointerMaps=01
; FRAME-TEXT-NEXT: {{.*}}ordinary safepoint{{.*}}map[1]{{.*}}ArgsPointerMaps=00{{.*}}LocalsPointerMaps=01

; OBJVIEW-LABEL: "name": "aarch64_abi0_pointer_result"
; OBJVIEW: "args": 16
; OBJVIEW: "kind": "args_pointer_maps"
; OBJVIEW: "count": 1
; OBJVIEW-NEXT: "num_bits": 2
; OBJVIEW: "set_bits": [
; OBJVIEW-NEXT: 0
; OBJVIEW: "kind": "locals_pointer_maps"
; OBJVIEW: "count": 1
; OBJVIEW: "stack_map_index": -1
; OBJVIEW-NEXT: "relocation_type": "R_CALLARM64"

; OBJVIEW-LABEL: "name": "aarch64_stack_pointer_arg"
; OBJVIEW: "args": 136
; OBJVIEW: "kind": "args_pointer_maps"
; OBJVIEW: "count": 1
; OBJVIEW-NEXT: "num_bits": 17
; OBJVIEW: "set_bits": [
; OBJVIEW-NEXT: 0
; OBJVIEW: "kind": "locals_pointer_maps"
; OBJVIEW: "count": 1
; OBJVIEW: "stack_map_index": -1
; OBJVIEW-NEXT: "relocation_type": "R_CALLARM64"

; OBJVIEW-LABEL: "name": "aarch64_subword_homes"
; OBJVIEW: "args": 8
; OBJVIEW: "kind": "args_pointer_maps"
; OBJVIEW: "count": 1
; OBJVIEW-NEXT: "num_bits": 1
; OBJVIEW: "set_bits": null
; OBJVIEW: "stack_map_index": -1
; OBJVIEW-NEXT: "relocation_type": "R_CALLARM64"

; OBJVIEW-LABEL: "name": "aarch64_large_arg_home"
; OBJVIEW: "args": 32776
; OBJVIEW: "kind": "args_pointer_maps"
; OBJVIEW: "count": 1
; OBJVIEW-NEXT: "num_bits": 4097
; OBJVIEW: "set_bits": null
; OBJVIEW: "stack_map_index": -1
; OBJVIEW-NEXT: "relocation_type": "R_CALLARM64"

; ASM: TEXT aarch64_subword_homes(SB)
; ASM: MOVB R0, 8(RSP)
; ASM: MOVH R1, 10(RSP)
; ASM: MOVBU 8(RSP), R0
; ASM: MOVHU 10(RSP), R1
; ASM: TEXT aarch64_large_arg_home(SB)
; ASM: ADD $16, RSP, R27
; ASM: MOVD R0, 32760(R27)
; ASM: MOVD 32760(R27), R0

declare goabiinternal void @"runtime.GC"()

; Go indirect-call code words originate as uintptr values, not GC pointers.
; Materialize the callable pointer only at the call so EntryArgs contains the
; data pointer while ordinary statepoint liveness excludes the call-only code
; pointer.
define goabiinternal ptr @aarch64_pointer_and_code_live(
    i64 %callee_bits, ptr %pointer) #0 gc "goallc" {
entry:
  %callee = inttoptr i64 %callee_bits to ptr
  call goabiinternal void %callee()
  %value = load i8, ptr %pointer, align 1
  %used = icmp ne i8 %value, 0
  %result = select i1 %used, ptr %pointer, ptr %pointer
  ret ptr %result
}

define goabi0 ptr @"aarch64_abi0_pointer_result<ABI0>"(ptr %pointer) #0 gc "goallc" {
entry:
  %buf = alloca [8192 x i8], align 16
  %slot = getelementptr inbounds [8192 x i8], ptr %buf, i64 0, i64 8191
  store volatile i8 1, ptr %slot, align 1
  ret ptr %pointer
}

define goabiinternal ptr @aarch64_stack_pointer_arg(
    i64 %a0, i64 %a1, i64 %a2, i64 %a3,
    i64 %a4, i64 %a5, i64 %a6, i64 %a7,
    i64 %a8, i64 %a9, i64 %a10, i64 %a11,
    i64 %a12, i64 %a13, i64 %a14, i64 %a15,
    ptr %pointer) #0 gc "goallc" {
entry:
  %buf = alloca [8192 x i8], align 16
  %slot = getelementptr inbounds [8192 x i8], ptr %buf, i64 0, i64 8191
  store volatile i8 1, ptr %slot, align 1
  ret ptr %pointer
}

; Sub-word integer arguments occupy W registers, but their ABI homes retain
; their original one- and two-byte widths and offsets.
define goabiinternal void @aarch64_subword_homes(i8 %a, i16 %b) #0 gc "goallc" {
entry:
  call goabiinternal void @"runtime.GC"()
  ret void
}

; The i64 register argument's home starts beyond the 8-byte scaled-uimm12
; limit (32760), forcing the frameless morestack path to materialize SP+32776.
define goabiinternal i64 @aarch64_large_arg_home(
    [4096 x i64] %stackarg, i64 %regarg) #0 gc "goallc" {
entry:
  call goabiinternal void @"runtime.GC"()
  ret i64 %regarg
}

attributes #0 = { "frame-pointer"="non-leaf" }
