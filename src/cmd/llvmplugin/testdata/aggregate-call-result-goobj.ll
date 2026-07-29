target triple = "x86_64-unknown-linux-goobj"

%pair = type { ptr, i64 }

declare goabiinternal void @safepoint()
declare goabiinternal %pair @make_pair(ptr, i64)
declare goabiinternal void @leaf_consume_pair(%pair) #0

define goabiinternal ptr @aggregate_call_result_goobj(
    ptr %seed, i1 %take_call)
    "go-stack-growth-statepoint" gc "goallc" {
entry:
  %value = call goabiinternal %pair @make_pair(ptr %seed, i64 17)
  br i1 %take_call, label %call, label %skip

call:
  call goabiinternal void @safepoint()
  br label %merge

skip:
  br label %merge

merge:
  call goabiinternal void @leaf_consume_pair(%pair %value)
  %result = extractvalue %pair %value, 0
  ret ptr %result
}

attributes #0 = { "gc-leaf-function" }
