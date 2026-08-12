target triple = "x86_64-unknown-linux-goobj"

%pair = type { ptr, i64 }

declare goabiinternal void @safepoint()

define goabiinternal ptr @aggregate_conditional_relocation(i1 %take_call, %pair %value) gc "goallc" {
entry:
  br i1 %take_call, label %call, label %skip

call:
  call goabiinternal void @safepoint()
  br label %merge

skip:
  br label %merge

merge:
  %result = extractvalue %pair %value, 0
  ret ptr %result
}
