target triple = "x86_64-unknown-linux-goobj"

; ERROR: GoALLC statepoints require a single fixed entry-block pointer-containing alloca

declare goabiinternal void @safepoint()

define goabiinternal void @dynamic_pointer_alloca(i64 %count) gc "goallc" {
entry:
  %slot = alloca ptr, i64 %count, align 8
  call goabiinternal void @safepoint()
  ret void
}
