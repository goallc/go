target triple = "aarch64-unknown-linux-goobj"

declare goabiinternal void @safepoint()

define goabiinternal ptr @nilcheck_sroa(ptr returned %value) "go-stack-growth-statepoint" gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr null, ptr %slot, align 8
  %nilcheck = load volatile i8, ptr %slot, align 1, !annotation !0
  call goabiinternal void @safepoint()
  store ptr %value, ptr %slot, align 8
  %result = load ptr, ptr %slot, align 8
  ret ptr %result
}

!0 = !{!"goallc.nilcheck"}
