target triple = "x86_64-unknown-linux-goobj"

; A pointer that is simultaneously a current aggregate call-argument field,
; a live aggregate field after the call, and an independently live scalar is
; represented by one statepoint root and one relocated machine value.

; MIR-LABEL: name: inserted_leaf_in_call_argument
; MIR: stack:
; MIR-NEXT: - { id: 0,
; MIR-NOT: - { id: 1,
; MIR: body:
; MIR-COUNT-1: MOV64mr %stack.0
; MIR: STATEPOINT {{.*}}@consume_pair{{.*}}%stack.0
; MIR-NOT: %stack.1
; MIR-COUNT-1: MOV64rm %stack.0
; MIR: CALL64pcrel32 @leaf_consume_pair
; MIR: RET 0,

; MIR-AARCH64-LABEL: name: inserted_leaf_in_call_argument
; MIR-AARCH64: stack:
; MIR-AARCH64-NEXT: - { id: 0,
; MIR-AARCH64-NOT: - { id: 1,
; MIR-AARCH64: body:
; MIR-AARCH64-COUNT-1: STRXui {{.*}}%stack.0
; MIR-AARCH64: STATEPOINT {{.*}}@consume_pair{{.*}}%stack.0
; MIR-AARCH64-NOT: %stack.1
; MIR-AARCH64-COUNT-1: LDRXui %stack.0
; MIR-AARCH64: BL @leaf_consume_pair
; MIR-AARCH64: RET_ReallyLR

%pair = type { ptr, i64 }

declare goabiinternal void @consume_pair(%pair)
declare goabiinternal void @leaf_consume_pair(%pair) #0

define goabiinternal ptr @inserted_leaf_in_call_argument(
    ptr %pointer, i64 %number) gc "goallc" {
entry:
  %with_pointer = insertvalue %pair zeroinitializer, ptr %pointer, 0
  %value = insertvalue %pair %with_pointer, i64 %number, 1
  call goabiinternal void @consume_pair(%pair %value)
  call goabiinternal void @leaf_consume_pair(%pair %value)
  ret ptr %pointer
}

attributes #0 = { "gc-leaf-function" }
