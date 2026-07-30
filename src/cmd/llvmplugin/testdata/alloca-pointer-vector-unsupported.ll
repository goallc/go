target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @safepoint()

define goabiinternal void @pointer_vector_alloca() "go-stack-growth-statepoint" gc "goallc" {
entry:
  %slot = alloca <2 x ptr>, align 16
  call goabiinternal void @safepoint()
  ret void
}
