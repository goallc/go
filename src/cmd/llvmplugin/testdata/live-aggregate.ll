target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @callee()

define goabiinternal ptr @live_pointer_aggregate({ ptr, i64 } %value) gc "goallc" {
entry:
  call goabiinternal void @callee()
  %pointer = extractvalue { ptr, i64 } %value, 0
  ret ptr %pointer
}
