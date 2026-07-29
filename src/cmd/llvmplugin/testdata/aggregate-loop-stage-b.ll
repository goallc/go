target triple = "x86_64-unknown-linux-goobj"

%pair = type { ptr, i64 }

declare goabiinternal void @safepoint()

define goabiinternal ptr @aggregate_loop_relocation(i1 %take_call, i1 %again, %pair %value) "go-stack-growth-statepoint" gc "goallc" {
entry:
  br label %header

header:
  br i1 %take_call, label %call, label %skip

call:
  call goabiinternal void @safepoint()
  br label %latch

skip:
  br label %latch

latch:
  br i1 %again, label %header, label %exit

exit:
  %result = extractvalue %pair %value, 0
  ret ptr %result
}
