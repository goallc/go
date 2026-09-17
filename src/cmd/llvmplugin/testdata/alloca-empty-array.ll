target triple = "x86_64-unknown-linux-goobj"

; A zero-size array contributes no pointer slots, regardless of its outer
; dimensions. The two real pointers must retain their adjacent bitmap bits.
; IR-LABEL: define goabiinternal ptr @empty_array_frame(
; IR: "deopt"(i64 1195461697, ptr %slot, i64 17, i64 3, i64 1095519299

%frame = type { ptr, [1000000000 x [1000000000 x [0 x ptr]]], ptr }

declare goabiinternal void @use_frame(ptr)

define goabiinternal ptr @empty_array_frame(ptr %first, ptr %last) gc "goallc" {
entry:
  %slot = alloca %frame, align 8
  store ptr %first, ptr %slot, align 8
  %tail = getelementptr %frame, ptr %slot, i64 0, i32 2
  store ptr %last, ptr %tail, align 8
  call goabiinternal void @use_frame(ptr %slot)
  %result = load ptr, ptr %tail, align 8
  ret ptr %result
}
