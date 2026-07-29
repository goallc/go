target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @safepoint()

define goabiinternal ptr @fixed_vector(<2 x ptr> %value) "go-stack-growth-statepoint" gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  %result = extractelement <2 x ptr> %value, i32 0
  ret ptr %result
}
