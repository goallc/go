target triple = "aarch64-unknown-linux-goobj"

declare i64 @llvm.read_register.i64(metadata)

define goabiinternal i64 @aarch64_regtmp_range(i64 %v) #0 gc "goallc" {
entry:
  %before = add i64 %v, 1
  %tmp = call i64 @llvm.read_register.i64(metadata !0)
  %after = add i64 %before, %tmp
  ret i64 %after
}

define goabiinternal i8 @aarch64_frame_transition(i8 %v) #0 gc "goallc" {
entry:
  %frame = alloca [288 x i8], align 16
  %slot = getelementptr inbounds [288 x i8], ptr %frame, i64 0, i64 0
  store volatile i8 %v, ptr %slot, align 1
  %result = load volatile i8, ptr %slot, align 1
  ret i8 %result
}

attributes #0 = { "go-async-unsafe" }

!0 = !{!"x27"}

; OBJVIEW-LABEL: "name": "aarch64_regtmp_range"
; OBJVIEW: "size": [[#SIZE:]]
; OBJVIEW: "kind": "unsafe_point"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": 4
; OBJVIEW-NEXT: "value": -2
; OBJVIEW: "start": 4
; OBJVIEW-NEXT: "end": [[#SIZE]]
; OBJVIEW-NEXT: "value": -1
; OBJVIEW-LABEL: "name": "aarch64_frame_transition"
; OBJVIEW: "size": [[#FRAME_SIZE:]]
; OBJVIEW: "kind": "pcsp"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": [[#FRAME_COMMIT:]]
; OBJVIEW-NEXT: "value": 0
; OBJVIEW: "start": [[#FRAME_COMMIT]]
; OBJVIEW-NEXT: "end": [[#FRAME_RELEASE:]]
; OBJVIEW-NEXT: "value": 320
; OBJVIEW: "start": [[#FRAME_RELEASE]]
; OBJVIEW-NEXT: "end": [[#FRAME_SIZE]]
; OBJVIEW-NEXT: "value": 0
; OBJVIEW: "kind": "unsafe_point"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": [[#STACK_CHECK_END:]]
; OBJVIEW-NEXT: "value": -2
; OBJVIEW: "start": [[#STACK_CHECK_END]]
; OBJVIEW-NEXT: "end": [[#FRAME_SETUP_START:]]
; OBJVIEW-NEXT: "value": -1
; OBJVIEW: "start": [[#FRAME_SETUP_START]]
; OBJVIEW-NEXT: "end": [[#FRAME_COMMIT+4]]
; OBJVIEW-NEXT: "value": -2
; OBJVIEW: "start": [[#FRAME_COMMIT+4]]
; OBJVIEW-NEXT: "end": [[#FRAME_DESTROY_ADDR:]]
; OBJVIEW-NEXT: "value": -1
; OBJVIEW: "start": [[#FRAME_DESTROY_ADDR]]
; OBJVIEW-NEXT: "end": [[#FRAME_RELEASE]]
; OBJVIEW-NEXT: "value": -2
; OBJVIEW: "start": [[#FRAME_RELEASE]]
; OBJVIEW-NEXT: "end": [[#FRAME_SIZE]]
; OBJVIEW-NEXT: "value": -1

; A runtime definition may be renamed outside the runtime namespace. Its
; explicit policy must protect the entire body, not just stack transitions.
define goabiinternal i64 @other.runtime_entry(ptr %p) #1 gc "goallc" {
entry:
  %v = load volatile i64, ptr %p, align 8
  %result = add i64 %v, 1
  store volatile i64 %result, ptr %p, align 8
  ret i64 %result
}

attributes #1 = { "go-async-unsafe"="all" }

; OBJVIEW-LABEL: "name": "other.runtime_entry"
; OBJVIEW: "size": [[#RUNTIME_SIZE:]]
; OBJVIEW: "kind": "unsafe_point"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": [[#RUNTIME_SIZE]]
; OBJVIEW-NEXT: "value": -2
