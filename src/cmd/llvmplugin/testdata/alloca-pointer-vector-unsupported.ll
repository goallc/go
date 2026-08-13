target triple = "x86_64-unknown-linux-goobj"

; ERROR: GoALLC statepoints do not support pointer vectors in allocas

declare goabiinternal void @safepoint()

define goabiinternal void @pointer_vector_alloca() gc "goallc" {
entry:
  %slot = alloca <2 x ptr>, align 8
  call goabiinternal void @safepoint()
  ret void
}
