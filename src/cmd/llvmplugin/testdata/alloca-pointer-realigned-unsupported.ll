target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @safepoint()

define goabiinternal void @realigned_pointer_alloca() "go-stack-growth-statepoint" gc "goallc" {
entry:
  %slot = alloca ptr, align 32
  call goabiinternal void @safepoint()
  ret void
}
