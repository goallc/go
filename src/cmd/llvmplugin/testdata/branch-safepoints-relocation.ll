target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @left_callee()
declare goabiinternal void @right_callee()

define goabiinternal i64 @branch_safepoints(ptr %p, i1 %take_left) gc "goallc" {
entry:
  br i1 %take_left, label %left, label %right

left:
  call goabiinternal void @left_callee()
  br label %merge

right:
  call goabiinternal void @right_callee()
  br label %merge

merge:
  %value = load i64, ptr %p, align 8
  ret i64 %value
}
