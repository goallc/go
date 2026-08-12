target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @callee()

define goabiinternal i64 @selected_stack_address_live_across_call(i1 %choose_a) gc "goallc" {
entry:
  %a = alloca i64, align 8
  %b = alloca i64, align 8
  store i64 41, ptr %a, align 8
  store i64 42, ptr %b, align 8
  %selected = select i1 %choose_a, ptr %a, ptr %b
  call goabiinternal void @callee()
  %value = load i64, ptr %selected, align 8
  ret i64 %value
}
