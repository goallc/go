target triple = "x86_64-unknown-linux-goobj"

; OBJVIEW-TEXT-LABEL: TEXT defer_edge(SB)
; OBJVIEW-TEXT: R_CALL:runtime.deferreturn
; OBJVIEW-TEXT-NEXT: {{.*}}ordinary safepoint{{.*}}map[{{[0-9]+}}]

; OBJVIEW-TEXT-LABEL: TEXT defer_result(SB)
; OBJVIEW-TEXT: FUNCDATA_LocalsPointerMaps count=3 bits=2 map[0]=00 map[1]=11 map[2]=10
; OBJVIEW-TEXT-NOT: FUNCDATA_StackObjects
; OBJVIEW-TEXT: R_CALL:runtime.deferproc
; OBJVIEW-TEXT-NEXT: {{.*}}ordinary safepoint{{.*}}map[1]{{.*}}LocalsPointerMaps=11
; OBJVIEW-TEXT: R_CALL:runtime.panicmem
; OBJVIEW-TEXT-NEXT: {{.*}}ordinary safepoint{{.*}}map[2]{{.*}}LocalsPointerMaps=10
; OBJVIEW-TEXT: R_CALL:runtime.deferreturn
; OBJVIEW-TEXT-NEXT: {{.*}}ordinary safepoint{{.*}}map[2]{{.*}}LocalsPointerMaps=10

; OBJVIEW-TEXT-LABEL: TEXT defer_wrapper(SB)

; OBJVIEW-JSON-LABEL: "name": "defer_wrapper"
; OBJVIEW-JSON: "func_id": 23
; OBJVIEW-JSON-NEXT: "func_flags": 0

declare goabiinternal void @runtime.deferproc()
declare goabiinternal void @runtime.deferreturn()
declare goabiinternal void @runtime.panicmem()
declare void @llvm.go.defer.edge()
declare void @llvm.lifetime.start.p0(ptr captures(none))

define goabiinternal void @defer_edge() gc "goallc" {
entry:
  call goabiinternal void @runtime.deferproc()
  callbr void @llvm.go.defer.edge() to label %normal [label %recover]

normal:
  ret void

recover:
  call goabiinternal void @runtime.deferreturn()
  ret void
}

define goabiinternal ptr @defer_result(ptr %pointer) gc "goallc" {
entry:
  %result = alloca ptr, align 8, !goallc.defer_result !1
  call void @llvm.lifetime.start.p0(ptr %result)
  store volatile ptr null, ptr %result, align 8
  call goabiinternal void @runtime.deferproc()
  callbr void @llvm.go.defer.edge() to label %panic [label %recover]

panic:
  store volatile ptr %pointer, ptr %result, align 8
  call goabiinternal void @runtime.panicmem()
  unreachable

recover:
  call goabiinternal void @runtime.deferreturn()
  %value = load volatile ptr, ptr %result, align 8
  ret ptr %value
}

define goabiinternal void @defer_wrapper() !goobj.func.info !0 {
entry:
  ret void
}

!0 = !{i8 23, i8 0}
!1 = !{}
