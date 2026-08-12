target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @first_callee()
declare goabiinternal void @second_callee()

define goabiinternal i64 @sequential_conditional_safepoints(
    ptr %p, i1 %take_first, i1 %take_second)
    gc "goallc" {
entry:
  br i1 %take_first, label %first_call, label %first_skip

first_call:
  call goabiinternal void @first_callee()
  br label %first_merge

first_skip:
  br label %first_merge

first_merge:
  br i1 %take_second, label %second_call, label %second_skip

second_call:
  call goabiinternal void @second_callee()
  br label %second_merge

second_skip:
  br label %second_merge

second_merge:
  %value = load i64, ptr %p, align 8
  ret i64 %value
}
