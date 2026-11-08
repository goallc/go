; Share the home, even when recursive use-list traversal visits the last store
; first. Keep dynamic GEP indices at their uses, and reload across every call.
target triple = "x86_64-unknown-linux-goobj"
%pair = type { ptr, i64 }
%nested = type { %pair, %pair }
declare goabiinternal void @safepoint()
declare goabiinternal i64 @index()

; COUNT-LABEL: define goabiinternal void @stores(
; COUNT: = load
; COUNT: store i8 1
; COUNT-NOT: = load
; COUNT: store i8 2
;
; IR-LABEL: define goabiinternal void @stores(
; IR: call goabiinternal token {{.*}}@llvm.experimental.gc.statepoint
; IR: [[P:%[^ ]+]] = load volatile ptr
; IR: getelementptr i8, ptr [[P]], i64 %n
; IR: store i8 1
; IR-NOT: = load
; IR: %next = add i64 %n, 1
; IR: getelementptr i8, ptr [[P]], i64 %next
; IR: store i8 2
; IR: getelementptr i8, ptr %arg.statepoint.home, i64 16
; IR: load volatile ptr
; IR: store i8 3
; IR: call goabiinternal token {{.*}}@llvm.experimental.gc.statepoint
; IR: [[IDX:%[^ ]+]] = call i64 @llvm.experimental.gc.result
; IR: [[LATE:%[^ ]+]] = load volatile ptr
; IR: getelementptr i8, ptr [[LATE]], i64 [[IDX]]
; IR: store i8 4
; IR: ret void
define goabiinternal void @stores(%nested %arg, i64 %n) gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  %a = extractvalue %nested %arg, 0
  %p = extractvalue %pair %a, 0
  %b = extractvalue %nested %arg, 1, 0
  %x = getelementptr i8, ptr %p, i64 %n
  store i8 1, ptr %x
  %next = add i64 %n, 1
  %alias = extractvalue %nested %arg, 0, 0
  %y = getelementptr i8, ptr %alias, i64 %next
  store i8 2, ptr %y
  %z = getelementptr i8, ptr %b, i64 %n
  store i8 3, ptr %z
  %idx = call goabiinternal i64 @index()
  %late = extractvalue %nested %arg, 0, 0
  %w = getelementptr i8, ptr %late, i64 %idx
  store i8 4, ptr %w
  ret void
}

; Reload separately on predecessor edges; a reload in one successor must
; never supply a sibling or a PHI incoming value on another edge.
; IR-LABEL: define goabiinternal ptr @branches(
; IR: left:
; IR: load volatile ptr
; IR-NOT: = load
; IR: br label %merge
; IR: right:
; IR: load volatile ptr
; IR-NOT: = load
; IR: br label %merge
; IR: merge:
; IR: phi ptr
; IR: ret ptr
define goabiinternal ptr @branches(%pair %arg, i1 %which, i64 %n) gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  call goabiinternal void @safepoint()
  %p = extractvalue %pair %arg, 0
  %q = getelementptr i8, ptr %p, i64 %n
  br i1 %which, label %left, label %right
left:
  store i8 1, ptr %q
  br label %merge
right:
  store i8 2, ptr %q
  br label %merge
merge:
  %result = phi ptr [ %q, %left ], [ %q, %right ]
  ret ptr %result
}

; Even a GC-leaf call is a conservative cache boundary. Whole aggregate uses
; and address projections both reload after calls. Identical paths share loads.
declare goabiinternal void @leaf() "gc-leaf-function"
declare goabiinternal void @consume(%pair, %pair)
; IR-LABEL: define goabiinternal void @whole_and_leaf(
; IR: call goabiinternal token {{.*}}@llvm.experimental.gc.statepoint
; IR: call goabiinternal token {{.*}}@llvm.experimental.gc.statepoint
; IR: load volatile ptr
; IR: store i8 1
; IR-NOT: = load
; IR: store i8 2
; IR: call goabiinternal void @leaf()
; IR: load volatile ptr
; IR: store i8 3
; IR: [[WHOLE:%[^ ]+]] = load volatile %pair, ptr %arg.statepoint.home
; IR-NOT: = load
; IR: @consume{{.*}}%pair [[WHOLE]], %pair [[WHOLE]]
; IR: ret void
define goabiinternal void @whole_and_leaf(%pair %arg, i64 %n) gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  call goabiinternal void @safepoint()
  %p = extractvalue %pair %arg, 0
  %q = getelementptr i8, ptr %p, i64 %n
  store i8 1, ptr %q
  %q2 = getelementptr i8, ptr %q, i64 1
  store i8 2, ptr %q2
  call goabiinternal void @leaf()
  store i8 3, ptr %q2
  call goabiinternal void @consume(%pair %arg, %pair %arg)
  ret void
}
