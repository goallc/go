target triple = "x86_64-unknown-linux-goobj"

; Stack coloring can assign lifetime-disjoint allocas with different pointer
; layouts to the same storage. Live-only records describe the roots at one
; callsite, not function-wide StackObjects.
declare goabiinternal void @checkpoint()

define goabiinternal ptr @colored_roots(ptr %value) gc "goallc" {
entry:
  %home = alloca [2 x ptr], align 8
  %second = getelementptr [2 x ptr], ptr %home, i64 0, i64 1
  store ptr %value, ptr %home, align 8
  %first = call goabiinternal token (i64, i32, ptr, i32, i32, ...)
      @llvm.experimental.gc.statepoint.p0(
          i64 1, i32 0, ptr elementtype(void ()) @checkpoint,
          i32 0, i32 0, i32 0, i32 0)
      [ "deopt"(i64 1195461697, ptr %home, i64 17, i64 1,
                  i64 1095519299, i64 5),
        "gc-live"(ptr %home) ]
  %moved = load ptr, ptr %home, align 8
  store ptr %moved, ptr %second, align 8
  store ptr null, ptr %home, align 8
  %last = call goabiinternal token (i64, i32, ptr, i32, i32, ...)
      @llvm.experimental.gc.statepoint.p0(
          i64 2, i32 0, ptr elementtype(void ()) @checkpoint,
          i32 0, i32 0, i32 0, i32 0)
      [ "deopt"(i64 1195461697, ptr %home, i64 17, i64 2,
                  i64 1095519299, i64 5),
        "gc-live"(ptr %home) ]
  %result = load ptr, ptr %second, align 8
  ret ptr %result
}

declare token @llvm.experimental.gc.statepoint.p0(i64 immarg, i32 immarg, ptr, i32 immarg, i32 immarg, ...)

; OBJVIEW-NOT: "kind": "stack_objects"
; OBJVIEW: "name": "colored_roots"
; OBJVIEW-NOT: "kind": "stack_objects"
; OBJVIEW: "kind": "locals_pointer_maps"
; OBJVIEW-X86: "raw_hex": "0300000003000000000204"
; OBJVIEW-AArch64: "raw_hex": "0300000002000000000102"
; OBJVIEW-NOT: "kind": "stack_objects"
