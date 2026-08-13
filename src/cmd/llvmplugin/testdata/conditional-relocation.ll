target triple = "x86_64-unknown-linux-goobj"

; OBJVIEW-LABEL: "name": "conditional_safepoint"
; OBJVIEW: "size": [[#SIZE:]]
; OBJVIEW: "kind": "unsafe_point"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": [[#SIZE]]
; OBJVIEW-NEXT: "value": -1
; OBJVIEW: "kind": "stack_map_index"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": [[#SAFEPOINT_START:]]
; OBJVIEW-NEXT: "value": 0
; OBJVIEW: "start": [[#SAFEPOINT_START]]
; OBJVIEW-NEXT: "end": [[#ENTRY_DEPTH_START:]]
; OBJVIEW-NEXT: "value": 1
; OBJVIEW: "start": [[#ENTRY_DEPTH_START]]
; OBJVIEW-NEXT: "end": [[#SIZE]]
; OBJVIEW-NEXT: "value": 0
; OBJVIEW: "kind": "locals_pointer_maps"
; OBJVIEW: "count": 2
; OBJVIEW: "index": 0
; OBJVIEW-NEXT: "set_bits": null
; OBJVIEW: "index": 1
; OBJVIEW: "set_bits": [
; OBJVIEW-NEXT: 0
; OBJVIEW: "call_offset": [[#SAFEPOINT_START+1]]
; OBJVIEW: "stack_map_index": 1
; OBJVIEW: "stack_map_index": 0

declare goabiinternal void @callee()

define goabiinternal i64 @conditional_safepoint(ptr %p, i1 %take_call) gc "goallc" {
entry:
  br i1 %take_call, label %call, label %skip

call:
  call goabiinternal void @callee()
  br label %merge

skip:
  br label %merge

merge:
  %first = load i64, ptr %p, align 8
  %second = load i64, ptr %p, align 8
  %sum = add i64 %first, %second
  ret i64 %sum
}

define goabiinternal i64 @conditional_phi_edge_use(
    ptr %p, ptr %q, i1 %take_call)
    gc "goallc" {
entry:
  br i1 %take_call, label %call, label %skip

call:
  call goabiinternal void @callee()
  br label %merge

skip:
  br label %merge

merge:
  %selected = phi ptr [ %p, %call ], [ %q, %skip ]
  %value = load i64, ptr %selected, align 8
  ret i64 %value
}
