target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @callee()

define goabiinternal i64 @loop_relocation(
    ptr %p, i1 %take_call, i1 %again)
    gc "goallc" {
entry:
  br label %header

header:
  br i1 %take_call, label %call, label %skip

call:
  call goabiinternal void @callee()
  br label %latch

skip:
  br label %latch

latch:
  %value = load i64, ptr %p, align 8
  br i1 %again, label %header, label %exit

exit:
  ret i64 %value
}
