target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @safepoint()

define goabiinternal void @select_different_pointer_allocas(
    i1 %choose) gc "goallc" {
entry:
  %left = alloca ptr, align 8
  %right = alloca ptr, align 8
  %selected = select i1 %choose, ptr %left, ptr %right
  store ptr null, ptr %selected, align 8
  call goabiinternal void @safepoint()
  ret void
}
