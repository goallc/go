target triple = "x86_64-unknown-linux-goobj"

; OPEN-DEFER-LABEL: define goabiinternal ptr @open_defer(
; OPEN-DEFER-NOT: goallc.open_defer
; OPEN-DEFER-NOT: llvm.memset.inline
; OPEN-DEFER: i64 1196377158, i64 6, i64 2, ptr %bits, ptr %slots, i64 1178881863, i64 6{{.*}}"gc-live"({{.*}}ptr %slots
; OPEN-DEFER: i64 1196377158, i64 6, i64 2, ptr %bits, ptr %slots, i64 1178881863, i64 6{{.*}}"gc-live"({{.*}}ptr %slots
; OPEN-DEFER: ret ptr

; OBJVIEW: "name": "open_defer"
; OBJVIEW: "kind": "locals_pointer_maps"
; OBJVIEW: "count": 3
; OBJVIEW: "index": 1
; OBJVIEW: "set_bits": [
; OBJVIEW-NEXT: [[#SLOT:]],
; OBJVIEW-NEXT: [[#SLOT+1]]
; OBJVIEW: "index": 2
; OBJVIEW: "set_bits": [
; OBJVIEW-NEXT: [[#SLOT]],
; OBJVIEW-NEXT: [[#SLOT+1]]
; OBJVIEW: "index": 2
; OBJVIEW: "kind": "stack_objects"
; OBJVIEW: "pkg_kind": "invalid"
; OBJVIEW: "index": 3
; OBJVIEW: "kind": "inline_tree"
; OBJVIEW: "pkg_kind": "invalid"
; OBJVIEW: "index": 4
; OBJVIEW: "kind": "open_coded_defer"
; OBJVIEW: "raw_hex": "{{[0-9a-f]+}}"

declare goabiinternal void @safepoint()

define goabiinternal ptr @open_defer(ptr %value) gc "goallc" {
entry:
  %bits = alloca i8, align 1, !goallc.open_defer_bits !0
  %slots = alloca [2 x ptr], align 8, !goallc.open_defer_slots !1
  %slot0 = getelementptr i8, ptr %slots, i64 0
  %slot1 = getelementptr i8, ptr %slots, i64 8
  store volatile i8 0, ptr %bits, align 1
  store volatile ptr null, ptr %slot0, align 8
  store volatile ptr null, ptr %slot1, align 8
  call goabiinternal void @safepoint()
  store volatile ptr %value, ptr %slot0, align 8
  store volatile i8 1, ptr %bits, align 1
  call goabiinternal void @safepoint()
  %result = load volatile ptr, ptr %slot0, align 8
  ret ptr %result
}

!0 = !{}
!1 = !{i32 2}
