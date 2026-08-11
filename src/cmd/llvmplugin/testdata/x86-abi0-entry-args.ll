target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @use_three_pointers(ptr, ptr, ptr)

define goabi0 void @abi0_pointer_arguments(
    ptr %first, ptr %second, ptr %third) #0 gc "goallc" {
entry:
  call goabiinternal void @use_three_pointers(
      ptr %first, ptr %second, ptr %third)
  ret void
}

attributes #0 = { "frame-pointer"="non-leaf" "go-stack-growth-statepoint" }
