; ERROR: GoObj alloca ptrmap layout changes between statepoints

target triple = "x86_64-unknown-linux-goobj"
declare goabiinternal void @checkpoint()
declare token @llvm.experimental.gc.statepoint.p0(i64 immarg, i32 immarg, ptr, i32 immarg, i32 immarg, ...)

; An inactive record requires a function-wide StackObject, whose bitmap must
; be identical even at statepoints where its contents are active.
define goabiinternal void @changing_stack_object() gc "goallc" {
entry:
  %home = alloca [2 x ptr], align 8
  store [2 x ptr] zeroinitializer, ptr %home, align 8
  %first = call goabiinternal token (i64, i32, ptr, i32, i32, ...) @llvm.experimental.gc.statepoint.p0(i64 1, i32 0, ptr elementtype(void ()) @checkpoint, i32 0, i32 0, i32 0, i32 0) [ "deopt"(i64 1195461697, ptr %home, i64 16, i64 1, i64 1095519299, i64 5) ]
  %last = call goabiinternal token (i64, i32, ptr, i32, i32, ...) @llvm.experimental.gc.statepoint.p0(i64 2, i32 0, ptr elementtype(void ()) @checkpoint, i32 0, i32 0, i32 0, i32 0) [ "deopt"(i64 1195461697, ptr %home, i64 17, i64 2, i64 1095519299, i64 5), "gc-live"(ptr %home) ]
  ret void
}
