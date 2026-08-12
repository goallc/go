target triple = "x86_64-unknown-linux-goobj"

; ERROR: GoALLC statepoints require a single fixed entry-block pointer-containing alloca

declare goabiinternal void @safepoint()

define goabiinternal void @nonentry_pointer_alloca(
    i1 %allocate) gc "goallc" {
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
