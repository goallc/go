target triple = "aarch64-apple-darwin-goobj"

; O2-COUNT-2: call i64 @llvm.go.pointer.address.i64.p0
; O2-NOT: ret i1 true

; REWRITE-LABEL: define goabiinternal i1 @observe(
; REWRITE-NOT: llvm.go.pointer.address
; REWRITE: %before.lowered = ptrtoint ptr %pointer to i64
; REWRITE: %pointer.relocated = call coldcc ptr @llvm.experimental.gc.relocate
; REWRITE: %after.lowered = ptrtoint ptr %pointer.relocated to i64
; REWRITE: %same = icmp eq i64 %before.lowered, %after.lowered

declare i64 @llvm.go.pointer.address.i64.p0(ptr)
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
