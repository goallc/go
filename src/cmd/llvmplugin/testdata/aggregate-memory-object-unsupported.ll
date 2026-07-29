target triple = "x86_64-unknown-linux-goobj"

%pair = type { ptr, i64 }

declare goabiinternal void @safepoint()

define goabiinternal ptr @pointer_slot_in_alloca(ptr %pointer) "go-stack-growth-statepoint" gc "goallc" {
entry:
  %slot = alloca %pair, align 8
  %value = insertvalue %pair poison, ptr %pointer, 0
  store %pair %value, ptr %slot, align 8
  call goabiinternal void @safepoint()
  %reloaded = load %pair, ptr %slot, align 8
  %result = extractvalue %pair %reloaded, 0
  ret ptr %result
}
