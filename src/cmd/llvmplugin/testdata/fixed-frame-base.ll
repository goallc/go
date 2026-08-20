target triple = "aarch64-apple-darwin-goobj"

%result_storage = type { ptr, i64, ptr }

; IR-LABEL: define goabiinternal {{.*}} @fixed_frame_goret_base(
; IR-COUNT-2: "gc-live"(ptr %result)

; MIR-LABEL: name: fixed_frame_goret_base
; MIR: fixedStack:
; MIR: type: default
; MIR: stack: []
; MIR: bb.0.entry:
; MIR: STATEPOINT{{.*}}%fixed-stack.0{{.*}}%fixed-stack.0
; MIR: bb.1.entry.statepoint.cont:
; MIR: STATEPOINT{{.*}}%fixed-stack.0{{.*}}%fixed-stack.0
; MIR: bb.2.entry.statepoint.cont.statepoint.cont:
; MIR: [[FRAME:%[0-9]+]]:gpr64sp = ADDXri %fixed-stack.0, 0, 0
; MIR: [[RESULT:%[0-9]+]]:gpr64sp = COPY [[FRAME]]
; MIR: STRXui {{.*}}, [[RESULT]], 0

; OBJVIEW-LABEL: "name": "fixed_frame_goret_base"
; OBJVIEW: "kind": "locals_pointer_maps"
; OBJVIEW: "count": 1
; OBJVIEW: "set_bits": null

declare goabiinternal void @safepoint()

define goabiinternal { i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 } @fixed_frame_goret_base(
    ptr goret(%result_storage) align 8 "goretindex"="15" %result) #0 gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  call goabiinternal void @safepoint()
  store ptr null, ptr %result, align 8
  ret { i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 } zeroinitializer
}

attributes #0 = { "frame-pointer"="non-leaf" "go_results_tuple" }
