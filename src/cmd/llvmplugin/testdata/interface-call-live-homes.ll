target triple = "x86_64-unknown-linux-goobj"

%iface = type { ptr, ptr }
%slice = type { ptr, i64, i64 }

; IR-LABEL: define goabiinternal i64 @interface_call_loop(
; IR-DAG: %left.statepoint.home = alloca %iface
; IR-DAG: %right.statepoint.home = alloca %iface
; IR-DAG: %scratch.statepoint.home = alloca %slice
; IR-DAG: store %iface %left, ptr %left.statepoint.home
; IR-DAG: store %iface %right, ptr %right.statepoint.home
; IR-DAG: store %slice %scratch, ptr %scratch.statepoint.home
; IR-NOT: extractvalue %iface %left
; IR-NOT: extractvalue %iface %right
; IR: load %iface, ptr %left.statepoint.home
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %scratch.statepoint.home, ptr %right.statepoint.home, ptr %left.statepoint.home)
; IR-NOT: @llvm.experimental.gc.relocate
; IR: load %iface, ptr %right.statepoint.home
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %scratch.statepoint.home, ptr %right.statepoint.home, ptr %left.statepoint.home)
; IR-NOT: @llvm.experimental.gc.relocate
; IR: load %slice, ptr %scratch.statepoint.home
; IR: ret i64

; MIR-LABEL: name: interface_call_loop
; MIR: fixedStack:
; MIR-DAG: size: 16
; MIR-DAG: size: 16
; MIR-DAG: size: 24
; MIR: stack: []
; MIR: STATEPOINT
; MIR: STATEPOINT

; MIR-AARCH64-LABEL: name: interface_call_loop
; MIR-AARCH64: fixedStack:
; MIR-AARCH64-DAG: size: 16
; MIR-AARCH64-DAG: size: 16
; MIR-AARCH64-DAG: size: 24
; MIR-AARCH64: stack: []
; MIR-AARCH64: STATEPOINT
; MIR-AARCH64: STATEPOINT

define goabiinternal i64 @interface_call_loop(
    %iface %left, %iface %right, %slice %scratch, i64 %limit) gc "goallc" {
entry:
  %left.itab = extractvalue %iface %left, 0
  %left.method = getelementptr i8, ptr %left.itab, i64 24
  %right.itab = extractvalue %iface %right, 0
  %right.method = getelementptr i8, ptr %right.itab, i64 24
  br label %loop

loop:
  %index = phi i64 [ 0, %entry ], [ %next, %loop ]
  %sum = phi i64 [ 0, %entry ], [ %updated, %loop ]
  %left.fn = load ptr, ptr %left.method, align 8
  %left.data = extractvalue %iface %left, 1
  %left.value = call goabiinternal i64 %left.fn(ptr %left.data, i64 %index, i64 %sum)
  %right.fn = load ptr, ptr %right.method, align 8
  %right.data = extractvalue %iface %right, 1
  %right.value = call goabiinternal i64 %right.fn(ptr %right.data, i64 %sum, i64 %index)
  %scratch.data = extractvalue %slice %scratch, 0
  %scratch.element = getelementptr i64, ptr %scratch.data, i64 %index
  %prior = load i64, ptr %scratch.element, align 8
  %partial = add i64 %left.value, %right.value
  %updated = add i64 %partial, %prior
  store i64 %updated, ptr %scratch.element, align 8
  %next = add i64 %index, 1
  %done = icmp eq i64 %next, %limit
  br i1 %done, label %exit, label %loop

exit:
  ret i64 %updated
}
