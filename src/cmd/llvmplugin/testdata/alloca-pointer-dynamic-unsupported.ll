target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @safepoint()

define goabiinternal void @dynamic_pointer_alloca(i64 %count) "go-stack-growth-statepoint" gc "goallc" {
entry:
  %slot = alloca ptr, i64 %count, align 8
  call goabiinternal void @safepoint()
  ret void
}
