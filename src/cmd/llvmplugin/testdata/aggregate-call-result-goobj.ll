target triple = "x86_64-unknown-linux-goobj"

; OBJVIEW-LABEL: "name": "aggregate_call_result_goobj"
; OBJVIEW: "size": [[#SIZE:]]
; OBJVIEW: "kind": "stack_map_index"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": [[#MAKE_START:]]
; OBJVIEW-NEXT: "value": -1
; OBJVIEW: "start": [[#MAKE_START]]
; OBJVIEW-NEXT: "end": [[#SAFEPOINT_START:]]
; OBJVIEW-NEXT: "value": 1
; OBJVIEW: "start": [[#SAFEPOINT_START]]
; OBJVIEW-NEXT: "end": [[#MORESTACK_START:]]
; OBJVIEW-NEXT: "value": 2
; OBJVIEW: "start": [[#MORESTACK_START]]
; OBJVIEW-NEXT: "end": [[#SIZE]]
; OBJVIEW-NEXT: "value": 0
; OBJVIEW: "kind": "args_pointer_maps"
; OBJVIEW: "count": 3
; OBJVIEW-NEXT: "num_bits": 2
; OBJVIEW: "index": 0
; OBJVIEW: "set_bits": [
; OBJVIEW-NEXT: 0
; OBJVIEW: "index": 1
; OBJVIEW-NEXT: "set_bits": null
; OBJVIEW: "index": 2
; OBJVIEW-NEXT: "set_bits": null
; OBJVIEW: "kind": "locals_pointer_maps"
; OBJVIEW: "count": 3
; OBJVIEW-NEXT: "num_bits": 4
; OBJVIEW: "index": 0
; OBJVIEW-NEXT: "set_bits": null
; OBJVIEW: "index": 1
; OBJVIEW-NEXT: "set_bits": null
; OBJVIEW: "index": 2
; OBJVIEW: "set_bits": [
; OBJVIEW-NEXT: 2
; OBJVIEW: "call_offset": [[#MAKE_START+1]]
; OBJVIEW: "return_pc": [[#MAKE_START+5]]
; OBJVIEW-NEXT: "lookup_pc": [[#MAKE_START+4]]
; OBJVIEW-NEXT: "stack_map_index": 1
; OBJVIEW-NEXT: "relocation_type": "R_CALL"
; OBJVIEW: "sym_index": 0
; OBJVIEW: "call_offset": [[#SAFEPOINT_START+1]]
; OBJVIEW: "stack_map_index": 2
; OBJVIEW-NEXT: "relocation_type": "R_CALL"
; OBJVIEW: "sym_index": 1
; OBJVIEW: "stack_map_index": 2
; OBJVIEW-NEXT: "relocation_type": "R_CALL"
; OBJVIEW: "sym_index": 2
; OBJVIEW: "call_offset": [[#MORESTACK_START+1]]
; OBJVIEW: "stack_map_index": 0
; OBJVIEW-NEXT: "relocation_type": "R_CALL"
; OBJVIEW: "sym_index": 3
; OBJVIEW: "references": [
; OBJVIEW: "class_index": 0
; OBJVIEW-NEXT: "name": "make_pair"
; OBJVIEW: "class_index": 1
; OBJVIEW-NEXT: "name": "safepoint"
; OBJVIEW: "class_index": 2
; OBJVIEW-NEXT: "name": "leaf_consume_pair"
; OBJVIEW: "class_index": 3
; OBJVIEW-NEXT: "name": "runtime.morestack_noctxt"

%pair = type { ptr, i64 }

declare goabiinternal void @safepoint()
declare goabiinternal %pair @make_pair(ptr, i64)
declare goabiinternal void @leaf_consume_pair(%pair) #0

define goabiinternal ptr @aggregate_call_result_goobj(
    ptr %seed, i1 %take_call)
    gc "goallc" {
entry:
  %value = call goabiinternal %pair @make_pair(ptr %seed, i64 17)
  br i1 %take_call, label %call, label %skip

call:
  call goabiinternal void @safepoint()
  br label %merge

skip:
  br label %merge

merge:
  call goabiinternal void @leaf_consume_pair(%pair %value)
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

attributes #0 = { "gc-leaf-function" }
