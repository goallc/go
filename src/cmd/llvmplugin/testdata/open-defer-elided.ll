target triple = "x86_64-unknown-linux-goobj"

; SROA may replace the slots aggregate after optimization proves that no defer
; can be registered. The replacement does not inherit the frontend metadata.
;
; OPEN-DEFER-ELIDED-LABEL: define goabiinternal void @open_defer_elided()
; OPEN-DEFER-ELIDED-NOT: goallc.open_defer
; OPEN-DEFER-ELIDED-NOT: i64 1196377158
; OPEN-DEFER-ELIDED: call goabiinternal token (i64, i32, ptr, i32, i32, ...)
; OPEN-DEFER-ELIDED: ret void

declare goabiinternal void @safepoint()

define goabiinternal void @open_defer_elided() gc "goallc" {
entry:
  %slots.sroa.0 = alloca ptr, align 8
  %bits = alloca i8, align 1, !goallc.open_defer_bits !0
  store volatile ptr null, ptr %slots.sroa.0, align 8
  store volatile i8 0, ptr %bits, align 1
  call goabiinternal void @safepoint()
  ret void
}

!0 = !{}
