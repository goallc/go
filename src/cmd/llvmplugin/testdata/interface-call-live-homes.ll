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

declare goabiinternal i64 @consume_pointer(ptr, i64)

; A scalar pointer is already one statepoint root, so homing it cannot reduce
; the root count. Even across two calls in a loop it stays on the relocate path;
; every derived address after a call is rebuilt from the corresponding
; relocate, never from the entry value.
;
; IR-LABEL: define goabiinternal i64 @scalar_pointer_call_loop(
; IR-NOT: statepoint.home
; IR: %entry.value = load i64, ptr %receiver
; IR: %[[MERGE:receiver.relocated.merge[.0-9]*]] = phi ptr
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %[[MERGE]])
; IR: %[[RELOC1:receiver.relocated[0-9]*]] = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %[[FIELD1:field.remat[0-9]*]] = getelementptr i64, ptr %[[RELOC1]]
; IR: %first.prior = load i64, ptr %[[FIELD1]]
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %[[RELOC1]])
; IR: %[[RELOC2:receiver.relocated[0-9]*]] = call coldcc ptr @llvm.experimental.gc.relocate
; IR: %[[FIELD2:field.remat[0-9]*]] = getelementptr i64, ptr %[[RELOC2]]
; IR: %second.prior = load i64, ptr %[[FIELD2]]
; IR: getelementptr i64, ptr %[[RELOC2]], i64 %index
; IR: ret i64
;
; MIR-LABEL: name: scalar_pointer_call_loop
; MIR: fixedStack:
; MIR-DAG: size: 8
; MIR-DAG: size: 8
; MIR: stack:
; MIR: size: 8
; MIR: STATEPOINT
; MIR: STATEPOINT
;
; MIR-AARCH64-LABEL: name: scalar_pointer_call_loop
; MIR-AARCH64: fixedStack:
; MIR-AARCH64-DAG: size: 8
; MIR-AARCH64-DAG: size: 8
; MIR-AARCH64: stack:
; MIR-AARCH64: size: 8
; MIR-AARCH64: STATEPOINT
; MIR-AARCH64: STATEPOINT
define goabiinternal i64 @scalar_pointer_call_loop(
    ptr %receiver, i64 %limit) gc "goallc" {
entry:
  %entry.value = load i64, ptr %receiver, align 8
  %field = getelementptr i64, ptr %receiver, i64 1
  br label %loop

loop:
  %index = phi i64 [ 0, %entry ], [ %next, %loop ]
  %first = call goabiinternal i64 @consume_pointer(ptr %receiver, i64 %index)
  %first.prior = load i64, ptr %field, align 8
  %second = call goabiinternal i64 @consume_pointer(ptr %receiver, i64 %first.prior)
  %second.prior = load i64, ptr %field, align 8
  %element = getelementptr i64, ptr %receiver, i64 %index
  %prior = load i64, ptr %element, align 8
  %partial = add i64 %first, %second
  %field.sum = add i64 %first.prior, %second.prior
  %called.sum = add i64 %partial, %field.sum
  %updated = add i64 %called.sum, %prior
  %next = add i64 %index, 1
  %done = icmp eq i64 %next, %limit
  br i1 %done, label %exit, label %loop

exit:
  %result = add i64 %updated, %entry.value
  ret i64 %result
}

; A single acyclic crossing does not amortize populating an ABI home. Keep the
; native scalar statepoint relocation and use only the relocated address.
;
; IR-LABEL: define goabiinternal i64 @single_call_pointer_relocate(
; IR-NOT: statepoint.home
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"(ptr %receiver)
; IR: %receiver.relocated = call coldcc ptr @llvm.experimental.gc.relocate
; IR: getelementptr i64, ptr %receiver.relocated
; IR: ret i64
define goabiinternal i64 @single_call_pointer_relocate(ptr %receiver)
    gc "goallc" {
entry:
  %called = call goabiinternal i64 @consume_pointer(ptr %receiver, i64 0)
  %element = getelementptr i64, ptr %receiver, i64 1
  %prior = load i64, ptr %element, align 8
  %updated = add i64 %called, %prior
  ret i64 %updated
}

; The same scalar policy holds beside a goret result carrier: two crossings
; relocate the pointer and do not make either fixed result/argument layout
; observable through a new scalar home.
;
; IR-LABEL: define goabiinternal void @goret_pointer_relocate(
; IR-NOT: %receiver.statepoint.home
; IR: @llvm.experimental.gc.statepoint
; IR-SAME: "gc-live"(ptr %receiver
; IR: %[[RELOC1:receiver.relocated[0-9]*]] = call coldcc ptr @llvm.experimental.gc.relocate
; IR: @llvm.experimental.gc.statepoint
; IR-SAME: "gc-live"(ptr %[[RELOC1]]
; IR: %[[RELOC2:receiver.relocated[0-9]*]] = call coldcc ptr @llvm.experimental.gc.relocate
; IR: load i64, ptr %[[RELOC2]]
; IR: ret void
define goabiinternal void @goret_pointer_relocate(
    ptr %receiver,
    ptr nofree writeonly goret([16 x i8]) align 1 captures(none) initializes((0, 16))
        "goretindex"="0" %result) gc "goallc" {
entry:
  %first = call goabiinternal i64 @consume_pointer(ptr %receiver, i64 0)
  %element = getelementptr i64, ptr %receiver, i64 %first
  %prior = load i64, ptr %element, align 8
  %second = call goabiinternal i64 @consume_pointer(ptr %receiver, i64 %prior)
  %final = load i64, ptr %receiver, align 8
  %sum = add i64 %second, %final
  %sum.byte = trunc i64 %sum to i8
  %result.value = insertvalue [16 x i8] zeroinitializer, i8 %sum.byte, 0
  store [16 x i8] %result.value, ptr %result, align 1
  ret void
}

; Apply the same profitability boundary to an FCA argument. It is scalarized
; into pointer leaves for one cold crossing, never turned into a whole-value
; relocated definition or a persistent home.
;
; IR-LABEL: define goabiinternal ptr @single_call_aggregate_relocate(
; IR-NOT: statepoint.home
; IR: @llvm.experimental.gc.statepoint{{.*}}"gc-live"({{(ptr %value.leaf.0, ptr %value.leaf.1|ptr %value.leaf.1, ptr %value.leaf.0)}})
; IR-DAG: %value.leaf.0.relocated = call coldcc ptr @llvm.experimental.gc.relocate
; IR-DAG: %value.leaf.1.relocated = call coldcc ptr @llvm.experimental.gc.relocate
; IR-NOT: relocated.merge
; IR: ret ptr
define goabiinternal ptr @single_call_aggregate_relocate(%iface %value)
    gc "goallc" {
entry:
  %ignored = call goabiinternal i64 @consume_pointer(ptr null, i64 0)
  %itab = extractvalue %iface %value, 0
  %method = getelementptr i8, ptr %itab, i64 24
  %method.value = load ptr, ptr %method, align 8
  %data = extractvalue %iface %value, 1
  %method.nil = icmp eq ptr %method.value, null
  %selected = select i1 %method.nil, ptr %data, ptr %method.value
  ret ptr %selected
}
