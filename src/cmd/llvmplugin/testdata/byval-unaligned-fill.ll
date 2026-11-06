target triple = "aarch64-unknown-linux-goobj"

; Reduced from pprof's webArgs construction. Skipping the bool at offset 112
; leaves a zero-fill range starting at 113. AArch64 CodeGenPrepare can hoist
; the high-offset address into entry, making this object cease to qualify as
; a direct byval carrier. Its fills must remain valid for GC content liveness:
; do not split the pointer at offset 128 into stores [113,129) and [129,145).
%Args = type { [112 x i8], i8, ptr, ptr, [72 x i8], ptr, i64, ptr, i64, i64, [80 x i8] }

declare void @llvm.memset.p0.i64(ptr, i8, i64, i1 immarg)
declare void @llvm.lifetime.start.p0(ptr)
declare goabiinternal void @before()
declare goabiinternal void @consume(ptr byval(%Args) align 8)

; IR-LABEL: define goabiinternal void @odd_offset_pointer_fill(
; IR: getelementptr i8, ptr %slot, i64 113
; IR-NEXT: store i8 0,
; IR: getelementptr i8, ptr %slot, i64 114
; IR-NEXT: store <2 x i8> zeroinitializer,
; IR: getelementptr i8, ptr %slot, i64 116
; IR-NEXT: store <4 x i8> zeroinitializer,
; IR: getelementptr i8, ptr %slot, i64 120
; IR-NEXT: store <8 x i8> zeroinitializer,
; IR: getelementptr i8, ptr %slot, i64 128
; IR-NEXT: store <16 x i8> zeroinitializer,
; IR: elementtype(void (ptr)) @consume
; MIR-LABEL: name: odd_offset_pointer_fill
; MIR: body:
; MIR: @consume
define goabiinternal void @odd_offset_pointer_fill(i1 %run, ptr %p, i64 %n) gc "goallc" {
entry:
  %slot = alloca %Args, align 8
  call goabiinternal void @before()
  br i1 %run, label %init, label %exit
init:
  call void @llvm.lifetime.start.p0(ptr %slot)
  call void @llvm.memset.p0.i64(ptr align 8 %slot, i8 0, i64 328, i1 false)
  %flag = getelementptr i8, ptr %slot, i64 112
  store i8 1, ptr %flag, align 8
  %p208 = getelementptr i8, ptr %slot, i64 208
  store ptr %p, ptr %p208, align 8
  %p216 = getelementptr i8, ptr %slot, i64 216
  store i64 %n, ptr %p216, align 8
  %p224 = getelementptr i8, ptr %slot, i64 224
  store ptr %p, ptr %p224, align 8
  %p232 = getelementptr i8, ptr %slot, i64 232
  store i64 %n, ptr %p232, align 8
  %p240 = getelementptr i8, ptr %slot, i64 240
  store i64 %n, ptr %p240, align 8
  call goabiinternal void @consume(ptr byval(%Args) align 8 %slot)
  ret void
exit:
  ret void
}
