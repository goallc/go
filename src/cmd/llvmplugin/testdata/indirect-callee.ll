target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @consume_pointer(ptr)

define goabiinternal void @indirect_callee(ptr %callee, i64 %arg) "go-stack-growth-statepoint" gc "goallc" {
entry:
  call goabiinternal void %callee(i64 %arg)
  ret void
}

define goabiinternal void @call_only_pointer_argument(ptr %value) "go-stack-growth-statepoint" gc "goallc" {
entry:
  call goabiinternal void @consume_pointer(ptr %value)
  ret void
}
