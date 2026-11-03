target triple = "x86_64-pc-windows-goobj"

declare goabi0 void @"probe.asm<ABI0>"()
declare i64 @llvm.read_register.i64(metadata)

define goabiinternal i64 @windows_g_reload() gc "goallc" {
entry:
  call goabi0 void @"probe.asm<ABI0>"()
  %g = call i64 @llvm.read_register.i64(metadata !0)
  ret i64 %g
}

!0 = !{!"r14"}

; The ABI0 return repairs R14 with three loads at offsets 15, 22 and 26:
; runtime.tls_g -> current thread's TLS block -> current g. The final load
; must not resume after migration with the previous thread's TLS address.
; Keep the call and ordinary body safe, but make the entire reload unsafe.
; OBJVIEW-LABEL: "name": "windows_g_reload"
; OBJVIEW: "kind": "unsafe_point"
; OBJVIEW: "start": 0
; OBJVIEW-NEXT: "end": 6
; OBJVIEW-NEXT: "value": -2
; OBJVIEW: "start": 6
; OBJVIEW-NEXT: "end": 15
; OBJVIEW-NEXT: "value": -1
; OBJVIEW: "start": 15
; OBJVIEW-NEXT: "end": 29
; OBJVIEW-NEXT: "value": -2
; OBJVIEW: "start": 29
; OBJVIEW-NEXT: "end": 33
; OBJVIEW-NEXT: "value": -1
