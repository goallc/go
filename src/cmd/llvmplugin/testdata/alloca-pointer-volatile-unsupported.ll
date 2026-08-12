target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @safepoint()

define goabiinternal void @volatile_pointer_alloca(ptr %pointer) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store volatile ptr %pointer, ptr %slot, align 8
  call goabiinternal void @safepoint()
  ret void
}
