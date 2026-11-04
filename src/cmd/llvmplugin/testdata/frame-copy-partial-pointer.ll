target triple = "aarch64-apple-darwin-goobj"
; ERROR: GoALLC cannot determine pointer alloca live-out activity
; A real partial pointer overwrite is still rejected. Preparing typed memory
; copies must not weaken the statepoint content-activity check.
declare goabiinternal void @checkpoint()
declare goabiinternal void @observe(ptr)
define goabiinternal void @partial() gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store i8 1, ptr %slot
  call goabiinternal void @checkpoint()
  call goabiinternal void @observe(ptr %slot)
  ret void
}
