target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @safepoint()

define goabiinternal void @zero_length_pointer_array()
    gc "goallc" {
entry:
  %slot = alloca [0 x ptr], align 8
  call goabiinternal void @safepoint()
  ret void
}
