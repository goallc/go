; IR-LABEL: define goabiinternal i1 @mixed_fields(
; IR: @llvm.memset.inline{{.*}}ptr align 8 %first
; IR: @llvm.memset.inline{{.*}}ptr align 8 %second
; IR: %frame.choice = phi i1
; IR-NEXT: %mixed.argument.base = phi ptr [ null, %stack ], [ %arg, %ordinary ]
; IR-NOT: phi ptr
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR-SAME: "deopt"(i64 1195461697, ptr %first, i64 9, i64 1, ptr %second, i64 9, i64 1,
; IR-SAME: "gc-live"(ptr %mixed.argument.base, ptr %second, ptr %first)
; IR-NOT: gc-live{{.*}}%first.field
; IR: mixed.frame.remat
; IR: load ptr
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR-SAME: "deopt"(i64 1195461697, ptr %first, i64 8, i64 1, ptr %second, i64 9, i64 1,
; IR: mixed.frame.remat
; IR: load ptr

; IR-LABEL: define goabiinternal i64 @grouped_pre_call_field(
; IR: %frame.choice = phi i1
; IR-NEXT: %mixed.argument.base = phi ptr [ null, %stack ], [ %arg, %ordinary ]
; IR-NOT: phi ptr
; IR: %[[BEFORE:.*]] = select i1 %frame.choice
; IR-NEXT: %x = load i64, ptr %[[BEFORE]]
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR-SAME: "gc-live"(ptr %mixed.argument.base, ptr %second)
; IR: %[[AFTER:.*]] = select i1 %frame.choice
; IR-NEXT: %y = load i64, ptr %[[AFTER]]

; IR-LABEL: define goabiinternal ptr @mixed_select_projection(
; IR: %frame.choice = freeze i1 %local
; IR-NEXT: %mixed.argument.base = select i1 %frame.choice, ptr null, ptr %arg
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR: mixed.frame.remat
; IR: load ptr

; IR-LABEL: define goabiinternal ptr @reverse_select(
; IR: %frame.choice = freeze i1 %ordinary
; IR: %[[CHOOSE:.*]] = xor i1 %frame.choice, true
; IR: %mixed.argument.base = select i1 %[[CHOOSE]], ptr null, ptr %arg
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR-SAME: "gc-live"(ptr %mixed.argument.base, ptr %frame)
; IR: mixed.frame.remat
; IR: load ptr

; IR-LABEL: define goabiinternal ptr @mixed_store(
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR-SAME: "deopt"(i64 1195461697, ptr %frame, i64 9, i64 1,
; IR: mixed.frame.remat
; IR: store ptr
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR-SAME: "deopt"(i64 1195461697, ptr %frame, i64 9, i64 1,
; IR: mixed.frame.remat
; IR: load ptr

; IR-LABEL: define goabiinternal ptr @escaping_merge(
; IR-NOT: mixed.frame.remat
; IR: ret ptr
; IR-LABEL: define goabiinternal ptr @ended_frame_merge(
; IR-NOT: mixed.frame.remat
; IR: @llvm.lifetime.end.p0
; IR-NOT: mixed.frame.remat
; IR: ret ptr
; IR-LABEL: define goabiinternal i64 @dynamic_offset(
; IR-NOT: mixed.frame.remat
; IR: ret i64
; IR-LABEL: define goabiinternal i64 @loop_merge(
; IR-NOT: mixed.frame.remat
; IR: ret i64
; IR-LABEL: define goabiinternal ptr @outside_merge_loop_use(
; IR: %frame.choice = freeze i1 %local
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR-SAME: "deopt"(i64 1195461697, ptr %frame, i64 9, i64 1,
; IR: mixed.frame.remat
; IR: load ptr

; IR-LABEL: define goabiinternal i64 @marked_live_nullable(
; IR: %field.base = select i1 %local, ptr %frame, ptr null
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR-SAME: "gc-live"(ptr %field.base)
; IR-LABEL: define goabiinternal i64 @call_free(
; IR-NOT: mixed.frame.remat
; IR: ret i64

target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @safepoint()
declare goabiinternal void @escape(ptr)
declare void @llvm.lifetime.start.p0(ptr)
declare void @llvm.lifetime.end.p0(ptr)

; Two separate field objects share the chosen edge and the ordinary argument,
; never a frame identity. Neither concrete selected address crosses a call.
define goabiinternal i1 @mixed_fields(ptr %arg, i1 %local, ptr %p, ptr %q) gc "goallc" {
entry:
  %first = alloca ptr, align 8
  %second = alloca ptr, align 8
  br i1 %local, label %stack, label %ordinary
stack:
  call void @llvm.lifetime.start.p0(ptr %first)
  call void @llvm.lifetime.start.p0(ptr %second)
  store ptr %p, ptr %first
  store ptr %q, ptr %second
  br label %merge
ordinary:
  %a = getelementptr i8, ptr %arg, i64 8
  %b = getelementptr i8, ptr %arg, i64 24
  br label %merge
merge:
  %first.field = phi ptr [ %first, %stack ], [ %a, %ordinary ]
  %second.field = phi ptr [ %second, %stack ], [ %b, %ordinary ]
  call goabiinternal void @safepoint()
  %x = load ptr, ptr %first.field
  call goabiinternal void @safepoint()
  %y = load ptr, ptr %second.field
  %equal = icmp eq ptr %x, %y
  ret i1 %equal
}

; Only the second field crosses the safepoint. Both fields share the same
; argument and incoming edge choice, including the first field's short life.
define goabiinternal i64 @grouped_pre_call_field(ptr %arg, i1 %local) gc "goallc" {
entry:
  %first = alloca i64, align 8
  %second = alloca i64, align 8
  store i64 7, ptr %first
  store i64 11, ptr %second
  br i1 %local, label %stack, label %ordinary
stack:
  br label %merge
ordinary:
  %a = getelementptr i8, ptr %arg, i64 8
  %b = getelementptr i8, ptr %arg, i64 24
  br label %merge
merge:
  %first.field = phi ptr [ %first, %stack ], [ %a, %ordinary ]
  %second.field = phi ptr [ %second, %stack ], [ %b, %ordinary ]
  %x = load i64, ptr %first.field
  call goabiinternal void @safepoint()
  %y = load i64, ptr %second.field
  %sum = add i64 %x, %y
  ret i64 %sum
}

; Follow a constant field projection as part of the address recipe.
define goabiinternal ptr @mixed_select_projection(ptr %arg, i1 %local, ptr %p) gc "goallc" {
  %frame = alloca {ptr, ptr}, align 8
  store {ptr, ptr} zeroinitializer, ptr %frame
  %frame.field = getelementptr i8, ptr %frame, i64 8
  store ptr %p, ptr %frame.field
  %base = select i1 %local, ptr %frame, ptr %arg
  %field = getelementptr i8, ptr %base, i64 8
  call goabiinternal void @safepoint()
  %v = load ptr, ptr %field
  ret ptr %v
}

; Both arm orders must mask the unused input and preserve a single choice.
define goabiinternal ptr @reverse_select(ptr %arg, i1 %ordinary, ptr %p) gc "goallc" {
  %frame = alloca ptr, align 8
  store ptr %p, ptr %frame
  %arg.field = getelementptr i8, ptr %arg, i64 8
  %field = select i1 %ordinary, ptr %arg.field, ptr %frame
  call goabiinternal void @safepoint()
  %v = load ptr, ptr %field
  ret ptr %v
}

; A conditional store is not a definite overwrite of the frame object.
; Its earlier contents must remain safe even on the other selected path.
define goabiinternal ptr @mixed_store(ptr %arg, i1 %local, ptr %p, ptr %q) gc "goallc" {
  %frame = alloca ptr, align 8
  store ptr %p, ptr %frame
  %arg.field = getelementptr i8, ptr %arg, i64 8
  %field = select i1 %local, ptr %frame, ptr %arg.field
  call goabiinternal void @safepoint()
  store ptr %q, ptr %field
  call goabiinternal void @safepoint()
  %v = load ptr, ptr %field
  ret ptr %v
}

; Escaping addresses keep the established root representation.
define goabiinternal ptr @escaping_merge(ptr %arg, i1 %local) gc "goallc" {
  %frame = alloca i64, align 8
  %arg.field = getelementptr i8, ptr %arg, i64 8
  %field = select i1 %local, ptr %frame, ptr %arg.field
  call goabiinternal void @escape(ptr %field)
  ret ptr %field
}

; Do not reactivate stale frame contents after the ordinary branch ended the
; object lifetime. This remains on the precise selected-pointer root path.
define goabiinternal ptr @ended_frame_merge(ptr %arg, i1 %local, ptr %p) gc "goallc" {
entry:
  %frame = alloca ptr, align 8
  call void @llvm.lifetime.start.p0(ptr %frame)
  store ptr %p, ptr %frame
  br i1 %local, label %stack, label %ordinary
stack:
  br label %merge
ordinary:
  call void @llvm.lifetime.end.p0(ptr %frame)
  %arg.field = getelementptr i8, ptr %arg, i64 8
  br label %merge
merge:
  %field = phi ptr [ %frame, %stack ], [ %arg.field, %ordinary ]
  call goabiinternal void @safepoint()
  %v = load ptr, ptr %field
  ret ptr %v
}

; Dynamic offsets and loop-carried merges remain on the existing path.
define goabiinternal i64 @dynamic_offset(ptr %arg, i1 %local, i64 %index) gc "goallc" {
  %frame = alloca i64, align 8
  store i64 7, ptr %frame
  %arg.field = getelementptr i8, ptr %arg, i64 %index
  %field = select i1 %local, ptr %frame, ptr %arg.field
  call goabiinternal void @safepoint()
  %v = load i64, ptr %field
  ret i64 %v
}

define goabiinternal i64 @loop_merge(ptr %arg, i1 %again) gc "goallc" {
entry:
  %frame = alloca i64, align 8
  store i64 7, ptr %frame
  br label %loop
loop:
  %field = phi ptr [ %frame, %entry ], [ %arg, %loop ]
  call goabiinternal void @safepoint()
  %v = load i64, ptr %field
  br i1 %again, label %loop, label %exit
exit:
  ret i64 %v
}

; An acyclic choice may feed memory uses in a loop. The frame contents still
; contribute roots on every iteration, even if the ordinary base gets a home.
define goabiinternal ptr @outside_merge_loop_use(ptr %arg, i1 %local, ptr %p, i1 %again) gc "goallc" {
entry:
  %frame = alloca ptr, align 8
  store ptr %p, ptr %frame
  %arg.field = getelementptr i8, ptr %arg, i64 8
  %field = select i1 %local, ptr %frame, ptr %arg.field
  br label %loop
loop:
  call goabiinternal void @safepoint()
  %v = load ptr, ptr %field
  br i1 %again, label %loop, label %exit
exit:
  ret ptr %v
}

; Legacy metadata is not provenance, even for a call-local memory address.
define goabiinternal i64 @marked_live_nullable(i1 %local) gc "goallc" {
  %frame = alloca [2 x i64], align 8
  store [2 x i64] zeroinitializer, ptr %frame
  %frame.field = getelementptr i8, ptr %frame, i64 8
  %field = select i1 %local, ptr %frame.field, ptr inttoptr (i64 8 to ptr), !goallc.mixed_frame_remat !0
  call goabiinternal void @safepoint()
  %v = load i64, ptr %field
  ret i64 %v
}

; No safepoint means there is no preservation work to eliminate.
define goabiinternal i64 @call_free(ptr %arg, i1 %local) gc "goallc" {
  %frame = alloca i64, align 8
  store i64 7, ptr %frame
  %field = select i1 %local, ptr %frame, ptr %arg
  %v = load i64, ptr %field
  ret i64 %v
}

; Keep independent argument groups separate even with interleaved PHIs and
; reversed incoming order. Their path selector can still be shared.
; IR-LABEL: define goabiinternal i64 @separate_groups(
; IR: %frame.choice = phi i1
; IR: %mixed.argument.base = phi ptr [ null, %stack ], [ %arg, %ordinary ]
; IR: %mixed.argument.base{{[0-9]+}} = phi ptr [ null, %stack ], [ %other, %ordinary ]
; IR-NOT: phi ptr
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR: select i1 %frame.choice
; IR: load i64
; IR: select i1 %frame.choice
; IR: load i64
; IR: select i1 %frame.choice
; IR: load i64
; IR: ret i64
define goabiinternal i64 @separate_groups(ptr %arg, ptr %other, i1 %local) gc "goallc" {
entry:
  %first = alloca i64, align 8
  %second = alloca i64, align 8
  %third = alloca i64, align 8
  store i64 1, ptr %first
  store i64 2, ptr %second
  store i64 3, ptr %third
  br i1 %local, label %stack, label %ordinary
stack:
  br label %merge
ordinary:
  %a = getelementptr i8, ptr %arg, i64 8
  %b = getelementptr i8, ptr %other, i64 16
  %c = getelementptr i8, ptr %arg, i64 24
  br label %merge
merge:
  %first.field = phi ptr [ %first, %stack ], [ %a, %ordinary ]
  %second.field = phi ptr [ %second, %stack ], [ %b, %ordinary ]
  %third.field = phi ptr [ %c, %ordinary ], [ %third, %stack ]
  call goabiinternal void @safepoint()
  %x = load i64, ptr %first.field
  %y = load i64, ptr %second.field
  %z = load i64, ptr %third.field
  %xy = add i64 %x, %y
  %xyz = add i64 %xy, %z
  ret i64 %xyz
}

; Sharing the path alone must not pull an unrelated short-lived argument
; group into rematerialization.
; IR-LABEL: define goabiinternal i64 @separate_call_free_group(
; IR: %mixed.argument.base = select i1 %frame.choice, ptr null, ptr %arg
; IR: %short.base = select i1 %local, ptr %second, ptr %other
; IR: %short.derived = getelementptr i8, ptr %short.base
; IR: %x = load i64, ptr %short.derived
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR: mixed.frame.remat
; IR: %y = load i64
; IR: ret i64
define goabiinternal i64 @separate_call_free_group(ptr %arg, ptr %other, i1 %local) gc "goallc" {
  %first = alloca i64, align 8
  %second = alloca i64, align 8
  store i64 1, ptr %first
  store i64 2, ptr %second
  %a = getelementptr i8, ptr %arg, i64 8
  %b = getelementptr i8, ptr %other, i64 16
  %long = select i1 %local, ptr %first, ptr %a
  %short = select i1 %local, ptr %second, ptr %b
  %x = load i64, ptr %short
  call goabiinternal void @safepoint()
  %y = load i64, ptr %long
  %sum = add i64 %x, %y
  ret i64 %sum
}

; Even an adjacent load cannot make input metadata a normalization exemption.
; IR-LABEL: define goabiinternal i64 @marked_local_nullable(
; IR: %field.base = select i1 %local, ptr %frame, ptr null
; IR: %field.derived = getelementptr i8, ptr %field.base, i64 8
; IR-NEXT: %v = load i64, ptr %field.derived
; IR: ret i64
define goabiinternal i64 @marked_local_nullable(i1 %local) gc "goallc" {
  %frame = alloca [2 x i64], align 8
  store [2 x i64] zeroinitializer, ptr %frame
  call goabiinternal void @safepoint()
  %frame.field = getelementptr i8, ptr %frame, i64 8
  %field = select i1 %local, ptr %frame.field, ptr inttoptr (i64 8 to ptr), !goallc.mixed_frame_remat !0
  %v = load i64, ptr %field
  ret i64 %v
}

!0 = !{}
