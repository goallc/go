target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @safepoint()

define goabiinternal ptr @open_defer(ptr %value) #0 gc "goallc" {
entry:
  %bits = alloca i8, align 1, !goallc.open_defer_bits !0
  %slots = alloca [2 x ptr], align 8, !goallc.open_defer_slots !1
  %slot0 = getelementptr i8, ptr %slots, i64 0
  %slot1 = getelementptr i8, ptr %slots, i64 8
  store volatile i8 0, ptr %bits, align 1
  store volatile ptr null, ptr %slot0, align 8
  store volatile ptr null, ptr %slot1, align 8
  call goabiinternal void @safepoint()
  store volatile ptr %value, ptr %slot0, align 8
  store volatile i8 1, ptr %bits, align 1
  call goabiinternal void @safepoint()
  %result = load volatile ptr, ptr %slot0, align 8
  ret ptr %result
}

attributes #0 = { "go-open-coded-defer" "go-stack-growth-statepoint" }

!0 = !{}
!1 = !{i32 2}
