target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @callee()

define goabiinternal i64 @ordinary_leaf(i64 %v) #0 gc "goallc" {
entry:
  %result = add i64 %v, 1
  ret i64 %result
}

define goabiinternal void @ordinary_call() #0 gc "goallc" {
entry:
  call goabiinternal void @callee()
  ret void
}

define goabiinternal i64 @ordinary_frame(i64 %v) #0 gc "goallc" {
entry:
  %slot = alloca i64, align 8
  store volatile i64 %v, ptr %slot, align 8
  %result = load volatile i64, ptr %slot, align 8
  ret i64 %result
}

define goabiinternal i64 @ordinary_vector_atomic(ptr %p, ptr %vectors) #0 gc "goallc" {
entry:
  %old = atomicrmw add ptr %p, i64 1 seq_cst, align 8
  %v = load <2 x i64>, ptr %vectors, align 8
  %result = add <2 x i64> %v, <i64 1, i64 1>
  store <2 x i64> %result, ptr %vectors, align 8
  ret i64 %old
}

define goabiinternal ptr @ordinary_pointer_roundtrip(ptr %p) #0 gc "goallc" {
entry:
  %address = ptrtoint ptr %p to i64
  %result = inttoptr i64 %address to ptr
  ret ptr %result
}

define goabiinternal i64 @whole_nosplit(i64 %v) #1 gc "goallc" {
entry:
  %result = add i64 %v, 1
  ret i64 %result
}

; Every function retains the frontend's fail-closed attribute. The final Go
; callback overrides it with precise ranges except where Go's nosplit policy
; deliberately keeps the complete function unsafe.
attributes #0 = { "go-async-unsafe" }
attributes #1 = { "go-async-unsafe" "go-nosplit" }

; OBJVIEW-LABEL: "name": "ordinary_leaf"
; OBJVIEW: "size": [[#LEAF_SIZE:]]
; OBJVIEW: "kind": "unsafe_point"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": [[#LEAF_SIZE]]
; OBJVIEW-NEXT: "value": -1
; OBJVIEW-LABEL: "name": "ordinary_call"
; OBJVIEW: "size": [[#CALL_SIZE:]]
; OBJVIEW: "kind": "unsafe_point"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": [[#CHECK_END:]]
; OBJVIEW-NEXT: "value": -2
; OBJVIEW: "start": [[#CHECK_END]]
; OBJVIEW-NEXT: "end": [[#BODY_END:]]
; OBJVIEW-NEXT: "value": -1
; OBJVIEW: "start": [[#BODY_END]]
; OBJVIEW-NEXT: "end": [[#MORESTACK_END:]]
; OBJVIEW-NEXT: "value": -2
; OBJVIEW: "start": [[#MORESTACK_END]]
; OBJVIEW-NEXT: "end": [[#CALL_SIZE]]
; OBJVIEW-NEXT: "value": -1
; OBJVIEW-LABEL: "name": "ordinary_frame"
; OBJVIEW: "size": [[#FRAME_SIZE:]]
; OBJVIEW: "kind": "unsafe_point"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": [[#FRAME_SIZE]]
; OBJVIEW-NEXT: "value": -1
; OBJVIEW-LABEL: "name": "ordinary_vector_atomic"
; OBJVIEW: "size": [[#VECTOR_ATOMIC_SIZE:]]
; OBJVIEW: "kind": "unsafe_point"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": [[#VECTOR_ATOMIC_SIZE]]
; OBJVIEW-NEXT: "value": -1
; OBJVIEW-LABEL: "name": "ordinary_pointer_roundtrip"
; OBJVIEW: "size": [[#POINTER_SIZE:]]
; OBJVIEW: "kind": "unsafe_point"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": [[#POINTER_SIZE]]
; OBJVIEW-NEXT: "value": -1
; OBJVIEW-LABEL: "name": "whole_nosplit"
; OBJVIEW: "size": [[#NOSPLIT_SIZE:]]
; OBJVIEW: "kind": "unsafe_point"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": [[#NOSPLIT_SIZE]]
; OBJVIEW-NEXT: "value": -2

; A runtime definition may be renamed outside the runtime namespace. Its
; explicit policy must protect the entire body, not just stack transitions.
define goabiinternal i64 @other.runtime_entry(ptr %p) #2 gc "goallc" {
entry:
  %v = load volatile i64, ptr %p, align 8
  %result = add i64 %v, 1
  store volatile i64 %result, ptr %p, align 8
  ret i64 %result
}

attributes #2 = { "go-async-unsafe"="all" }

; OBJVIEW-LABEL: "name": "other.runtime_entry"
; OBJVIEW: "size": [[#RUNTIME_SIZE:]]
; OBJVIEW: "kind": "unsafe_point"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": [[#RUNTIME_SIZE]]
; OBJVIEW-NEXT: "value": -2
