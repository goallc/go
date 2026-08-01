target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @safepoint()

define goabiinternal void @nonentry_pointer_alloca(
    i1 %allocate) "go-stack-growth-statepoint" gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  br i1 %allocate, label %allocate.block, label %exit

allocate.block:
  %slot = alloca ptr, align 8
  store ptr null, ptr %slot, align 8
  call goabiinternal void @safepoint()
  br label %exit

exit:
  ret void
}
