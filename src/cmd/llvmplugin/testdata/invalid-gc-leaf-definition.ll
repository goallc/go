target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @non_leaf()

define goabiinternal void @invalid_leaf() #0 gc "goallc" {
entry:
  call goabiinternal void @non_leaf()
  ret void
}

attributes #0 = { "gc-leaf-function" }
