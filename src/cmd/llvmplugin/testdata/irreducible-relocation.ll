target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @callee()

define goabiinternal i64 @irreducible_relocation(
    ptr %p, i1 %enter_b, i1 %leave_a, i1 %leave_b)
    gc "goallc" {
entry:
  br i1 %enter_b, label %b, label %a

a:
  call goabiinternal void @callee()
  br i1 %leave_a, label %exit, label %b

b:
  br i1 %leave_b, label %exit, label %a

exit:
  %value = load i64, ptr %p, align 8
  ret i64 %value
}
