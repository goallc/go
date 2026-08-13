target triple = "aarch64-unknown-linux-goobj"

declare goabiinternal void @supported_callee(ptr)

define goabiinternal void @supported_param_attrs(ptr %argument) #0 gc "goallc" {
entry:
  call goabiinternal void @supported_callee(ptr noundef nonnull readnone align 8 %argument)
  ret void
}

attributes #0 = { "frame-pointer"="non-leaf" }
