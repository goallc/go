target triple = "aarch64-apple-darwin-goobj"

; O2-LABEL: define goabiinternal i1 @observe(
; O2-COUNT-2: call i64 @llvm.go.pointer.address.i64.p0
; O2-NOT: ret i1 true

; O2-LABEL: define goabiinternal ptr @restore(
; O2: %pointer = {{(tail )?}}call ptr @llvm.go.pointer.from.address.p0.i64(i64 %address)
; O2-NEXT: {{(tail )?}}call goabiinternal void @callee()
; O2: ret ptr %pointer

; REWRITE-LABEL: define goabiinternal i1 @observe(
; REWRITE-NOT: llvm.go.pointer.address
; REWRITE: %before.lowered = ptrtoint ptr %pointer to i64
; REWRITE: %pointer.relocated = call coldcc ptr @llvm.experimental.gc.relocate
; REWRITE: %after.lowered = ptrtoint ptr %pointer.relocated to i64
; REWRITE: %same = icmp eq i64 %before.lowered, %after.lowered

; REWRITE-LABEL: define goabiinternal ptr @restore(
; REWRITE-NOT: llvm.go.pointer.from.address
; REWRITE: %pointer.lowered = inttoptr i64 %address to ptr
; REWRITE: %pointer.lowered.relocated = call coldcc ptr @llvm.experimental.gc.relocate
; REWRITE: ret ptr %pointer.lowered.relocated

declare i64 @llvm.go.pointer.address.i64.p0(ptr)
declare ptr @llvm.go.pointer.from.address.p0.i64(i64)
declare goabiinternal void @callee()

define goabiinternal i1 @observe(ptr %pointer)
    gc "goallc" {
entry:
  %before = call i64 @llvm.go.pointer.address.i64.p0(ptr %pointer)
  call goabiinternal void @callee()
  %after = call i64 @llvm.go.pointer.address.i64.p0(ptr %pointer)
  %same = icmp eq i64 %before, %after
  ret i1 %same
}

define goabiinternal ptr @restore(i64 %address) gc "goallc" {
entry:
  %pointer = call ptr @llvm.go.pointer.from.address.p0.i64(i64 %address)
  call goabiinternal void @callee()
  ret ptr %pointer
}
