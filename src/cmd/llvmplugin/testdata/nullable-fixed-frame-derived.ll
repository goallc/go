target triple = "x86_64-unknown-linux-goobj"

; IR-LABEL: define goabiinternal i8 @nullable_fixed_frame_derived(
; IR: %base = phi ptr [ null, %nil ], [ %frame, %present ]
; IR-NOT: phi ptr [ inttoptr (i64 2240 to ptr)
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR-SAME: "gc-live"(ptr %base)
; IR: %base.relocated{{.*}} = call {{.*}}ptr @llvm.experimental.gc.relocate
; IR: %field.derived.remat{{.*}} = getelementptr i8, ptr %base.relocated{{.*}}, i64 2240

; IR-LABEL: define goabiinternal ptr @ordinary_nullable_base(
; IR: %base = phi ptr [ null, %nil ], [ %heap, %present ]
; IR-NOT: %base.base
; IR-NOT: %base.derived
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR-SAME: "gc-live"(ptr %base)

; IR-LABEL: define goabiinternal ptr @nullable_heap_derived(
; IR: %field.base = phi ptr [ null, %nil ], [ %heap, %present ]
; IR-NOT: inttoptr (i64 32 to ptr)
; IR: %field.derived = getelementptr i8, ptr %field.base, i64 32
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR-SAME: "gc-live"(ptr %field.base)
; IR: %field.base.relocated{{.*}} = call {{.*}}ptr @llvm.experimental.gc.relocate
; IR: %field.derived.remat{{.*}} = getelementptr i8, ptr %field.base.relocated{{.*}}, i64 32

; IR-LABEL: define goabiinternal ptr @different_derived_offsets(
; IR: %field.base = phi ptr [ null, %nil ], [ %heap, %present ]
; IR: %field.offset = phi i64 [ 8, %nil ], [ 16, %present ]
; IR: %field.derived = getelementptr i8, ptr %field.base, i64 %field.offset
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR-SAME: "gc-live"(ptr %field.base)
; IR: %field.derived.remat{{.*}} = getelementptr i8, ptr %field.base.relocated{{.*}}, i64 %field.offset

; IR-LABEL: define goabiinternal ptr @selected_derived_pointer(
; IR: %field.base = select i1 %use_heap, ptr %heap, ptr null
; IR: %field.derived = getelementptr i8, ptr %field.base, i64 64
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR-SAME: "gc-live"(ptr %field.base)
; IR: %field.derived.remat{{.*}} = getelementptr i8, ptr %field.base.relocated{{.*}}, i64 64

; IR-LABEL: define goabiinternal ptr @loop_derived_pointer(
; IR: %heap.relocated.merge = phi ptr [ %heap.relocated, %backedge ], [ %heap, %entry ]
; IR: %field.offset = phi i64 [ 8, %entry ], [ %next.offset, %backedge ]
; IR: %field.derived = getelementptr i8, ptr %heap.relocated.merge, i64 %field.offset
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR-SAME: "gc-live"(ptr %heap.relocated.merge)
; IR: %heap.relocated{{.*}} = call {{.*}}ptr @llvm.experimental.gc.relocate
; IR: %field.derived.remat{{.*}} = getelementptr i8, ptr %heap.relocated{{.*}}, i64 %field.offset

; MIR-LABEL: name: nullable_fixed_frame_derived
; MIR: stack:
; MIR: name: frame{{.*}}size: 2248
; MIR-NOT: MOV{{.*}}2240
; MIR: STATEPOINT{{.*}}@safepoint{{.*}}1, 8, %stack.1
; MIR: MOV8rm{{.*}}2240

; MIR-AARCH64-LABEL: name: nullable_fixed_frame_derived
; MIR-AARCH64: stack:
; MIR-AARCH64: name: frame{{.*}}size: 2248
; MIR-AARCH64-NOT: MOVi32imm 2240
; MIR-AARCH64: STATEPOINT{{.*}}@safepoint{{.*}}1, 8, %stack.1
; MIR-AARCH64: LDRBBui{{.*}}2240

; OBJVIEW-LABEL: TEXT nullable_fixed_frame_derived(SB)
; OBJVIEW: size={{[0-9]+}} args=8 locals=2256
; OBJVIEW: FUNCDATA_LocalsPointerMaps count=2 bits=282
; OBJVIEW-SAME: map[0]={{0+}}
; OBJVIEW-SAME: map[1]=1{{0+}}
; OBJVIEW: CALL {{.*}}R_CALL:safepoint
; OBJVIEW-SAME: PCDATA_StackMapIndex=1
; OBJVIEW-SAME: LocalsPointerMaps=1{{0+}}
; OBJVIEW: MOVQ 0(SP), AX
; OBJVIEW-NEXT: {{.*}}MOVZX 0x8c0(AX), AX

declare goabiinternal void @safepoint()

; Match SROA's optimized form for a field address made through a pointer that
; selects either nil or a fixed stack object. The integer 2240 must remain an
; address offset, never an independently tracked Go pointer.
define goabiinternal i8 @nullable_fixed_frame_derived(i1 %use_frame)
    gc "goallc" {
entry:
  %frame = alloca [281 x i64], align 8
  br i1 %use_frame, label %present, label %nil

nil:
  br label %merge

present:
  %frame.field = getelementptr i8, ptr %frame, i64 2240
  br label %merge

merge:
  %field = phi ptr [ inttoptr (i64 2240 to ptr), %nil ],
                   [ %frame.field, %present ]
  %base = phi ptr [ null, %nil ], [ %frame, %present ]
  call goabiinternal void @safepoint()
  %value = load i8, ptr %field, align 1
  ret i8 %value
}

; A nullable merge of two base values is already a base. Do not manufacture a
; parallel PHI or turn it into a zero-offset derived pointer.
define goabiinternal ptr @ordinary_nullable_base(ptr %heap, i1 %use_heap)
    gc "goallc" {
entry:
  br i1 %use_heap, label %present, label %nil

nil:
  br label %merge

present:
  br label %merge

merge:
  %base = phi ptr [ null, %nil ], [ %heap, %present ]
  call goabiinternal void @safepoint()
  ret ptr %base
}

; Constants have null base, matching RewriteStatepointsForGC. This is the same
; base inference for a heap pointer as for a fixed frame object.
define goabiinternal ptr @nullable_heap_derived(ptr %heap, i1 %use_heap)
    gc "goallc" {
entry:
  br i1 %use_heap, label %present, label %nil

nil:
  br label %merge

present:
  %heap.field = getelementptr i8, ptr %heap, i64 32
  br label %merge

merge:
  %field = phi ptr [ inttoptr (i64 32 to ptr), %nil ],
                   [ %heap.field, %present ]
  call goabiinternal void @safepoint()
  ret ptr %field
}

; The pointer merge and its integer offset merge are independent. Different
; offsets therefore use the same inferred base mechanism without a PHI-shape
; special case.
define goabiinternal ptr @different_derived_offsets(ptr %heap, i1 %use_heap)
    gc "goallc" {
entry:
  br i1 %use_heap, label %present, label %nil

nil:
  br label %merge

present:
  %heap.field = getelementptr i8, ptr %heap, i64 16
  br label %merge

merge:
  %field = phi ptr [ inttoptr (i64 8 to ptr), %nil ],
                   [ %heap.field, %present ]
  call goabiinternal void @safepoint()
  ret ptr %field
}

; Selects are base-defining values under the same analysis and do not need a
; target-specific normalization path.
define goabiinternal ptr @selected_derived_pointer(ptr %heap, i1 %use_heap)
    gc "goallc" {
entry:
  %heap.field = getelementptr i8, ptr %heap, i64 64
  %field = select i1 %use_heap, ptr %heap.field,
                                  ptr inttoptr (i64 64 to ptr)
  call goabiinternal void @safepoint()
  ret ptr %field
}

; A loop-carried derived address exercises the optimistic BDV fixed point and
; the matching integer-offset SSA construction.
define goabiinternal ptr @loop_derived_pointer(ptr %heap, i1 %again)
    gc "goallc" {
entry:
  %start = getelementptr i8, ptr %heap, i64 8
  br label %loop

loop:
  %field = phi ptr [ %start, %entry ], [ %next, %backedge ]
  call goabiinternal void @safepoint()
  br i1 %again, label %backedge, label %exit

backedge:
  %next = getelementptr i8, ptr %field, i64 8
  br label %loop

exit:
  ret ptr %field
}
