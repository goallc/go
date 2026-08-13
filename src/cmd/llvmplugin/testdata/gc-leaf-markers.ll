target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @callee_leaf() "gc-leaf-function"
declare goabiinternal void @callsite_leaf()

define goabiinternal void @leaf_calls() gc "goallc" {
entry:
  call goabiinternal void @callee_leaf()
  call goabiinternal void @callsite_leaf() "gc-leaf-function"
  ret void
}
