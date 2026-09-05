target triple = "x86_64-unknown-linux-goobj"

; The content-active field after size/alignment/pointer-size is independent of
; gc-live address rematerialization and the function-wide StackObject layout.
; No VarDef annotations or repeated lifetime starts are needed for overwrites.
; IR-LABEL: define goabiinternal ptr @overwrite(
; IR: @checkpoint{{.*}}ptr %slot, i64 0, i64 8, i64 8, i64 8, i64 0,
; IR-LABEL: define goabiinternal ptr @overwrite_fields(
; IR: @checkpoint{{.*}}ptr %slot, i64 0, i64 16, i64 8, i64 8, i64 0,
; IR-LABEL: define goabiinternal ptr @partial_object(
; IR: @checkpoint{{.*}}ptr %slot, i64 0, i64 16, i64 8, i64 8, i64 1,
; IR-LABEL: define goabiinternal ptr @overlapping_move(
; IR: @checkpoint{{.*}}ptr %slot, i64 0, i64 24, i64 8, i64 8, i64 1,
; IR-LABEL: define goabiinternal ptr @self_copy(
; IR: @checkpoint{{.*}}ptr %slot, i64 0, i64 8, i64 8, i64 8, i64 1,
; IR-LABEL: define goabiinternal ptr @both_paths_overwrite(
; IR: @checkpoint{{.*}}ptr %slot, i64 0, i64 8, i64 8, i64 8, i64 0,
; IR-LABEL: define goabiinternal ptr @one_path_reads(
; IR: @checkpoint{{.*}}ptr %slot, i64 0, i64 8, i64 8, i64 8, i64 1,
; IR-LABEL: define goabiinternal void @loop_overwrite(
; IR: @checkpoint{{.*}}ptr %slot, i64 0, i64 8, i64 8, i64 8, i64 0,
; IR-LABEL: define goabiinternal ptr @unknown_write_offset(
; IR: @checkpoint{{.*}}ptr %slot, i64 0, i64 16, i64 8, i64 8, i64 1,
; IR-LABEL: define goabiinternal ptr @byval_self_copy(
; IR: @checkpoint{{.*}}ptr %slot, i64 0, i64 8, i64 8, i64 8, i64 1,

%pair = type { ptr, ptr }
declare goabiinternal void @observe(ptr)
declare goabiinternal void @checkpoint()
declare void @llvm.memset.inline.p0.i64(ptr, i8, i64, i1 immarg)
declare void @llvm.memmove.p0.p0.i64(ptr, ptr, i64, i1 immarg)

define goabiinternal ptr @overwrite(ptr %old) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %old, ptr %slot
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  store ptr null, ptr %slot
  %result = load ptr, ptr %slot
  ret ptr %result
}

define goabiinternal ptr @overwrite_fields(ptr %old) gc "goallc" {
entry:
  %slot = alloca %pair, align 8
  %second = getelementptr %pair, ptr %slot, i32 0, i32 1
  store ptr %old, ptr %slot
  store ptr %old, ptr %second
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  store ptr null, ptr %slot
  store ptr null, ptr %second
  %result = load ptr, ptr %second
  ret ptr %result
}

define goabiinternal ptr @partial_object(ptr %old) gc "goallc" {
entry:
  %slot = alloca %pair, align 8
  %second = getelementptr %pair, ptr %slot, i32 0, i32 1
  store ptr %old, ptr %slot
  store ptr %old, ptr %second
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  store ptr null, ptr %slot
  %result = load ptr, ptr %second
  ret ptr %result
}

define goabiinternal ptr @overlapping_move(ptr %old) gc "goallc" {
entry:
  %slot = alloca [3 x ptr], align 8
  %second = getelementptr [3 x ptr], ptr %slot, i64 0, i64 1
  %third = getelementptr [3 x ptr], ptr %slot, i64 0, i64 2
  call void @llvm.memset.inline.p0.i64(ptr %slot, i8 0, i64 24, i1 false)
  store ptr %old, ptr %slot
  store ptr %old, ptr %second
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  call void @llvm.memmove.p0.p0.i64(ptr %second, ptr %slot, i64 16, i1 false)
  %result = load ptr, ptr %third
  ret ptr %result
}

define goabiinternal ptr @self_copy(ptr %old) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %old, ptr %slot
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  call void @llvm.memmove.p0.p0.i64(ptr %slot, ptr %slot, i64 8, i1 false)
  %result = load ptr, ptr %slot
  ret ptr %result
}

define goabiinternal ptr @both_paths_overwrite(ptr %old, i1 %cond) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %old, ptr %slot
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  br i1 %cond, label %left, label %right
left:
  store ptr null, ptr %slot
  br label %merge
right:
  call void @llvm.memset.inline.p0.i64(ptr %slot, i8 0, i64 8, i1 false)
  br label %merge
merge:
  %result = load ptr, ptr %slot
  ret ptr %result
}

define goabiinternal ptr @one_path_reads(ptr %old, i1 %cond) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %old, ptr %slot
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  br i1 %cond, label %left, label %merge
left:
  store ptr null, ptr %slot
  br label %merge
merge:
  %result = load ptr, ptr %slot
  ret ptr %result
}

define goabiinternal void @loop_overwrite(ptr %old, i1 %again) gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  store ptr %old, ptr %slot
  br label %loop
loop:
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  store ptr null, ptr %slot
  br i1 %again, label %loop, label %exit
exit:
  ret void
}

define goabiinternal ptr @unknown_write_offset(ptr %old, i64 %index) gc "goallc" {
entry:
  %slot = alloca [2 x ptr], align 8
  call void @llvm.memset.inline.p0.i64(ptr %slot, i8 0, i64 16, i1 false)
  store ptr %old, ptr %slot
  call goabiinternal void @observe(ptr %slot)
  call goabiinternal void @checkpoint()
  %destination = getelementptr [2 x ptr], ptr %slot, i64 0, i64 %index
  store ptr null, ptr %destination
  %result = load ptr, ptr %slot
  ret ptr %result
}

; The shared transfer must not turn a same-address ABI-home copy into a kill.
define goabiinternal ptr @byval_self_copy(ptr byval(ptr) align 8 %slot) gc "goallc" {
entry:
  call goabiinternal void @checkpoint()
  call void @llvm.memmove.p0.p0.i64(ptr %slot, ptr %slot, i64 8, i1 false)
  %result = load ptr, ptr %slot
  ret ptr %result
}
