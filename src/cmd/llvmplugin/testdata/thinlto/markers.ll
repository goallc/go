; Preparation must precede AsmPrinter initialization, including in the standard
; ThinLTO codegen path. Both marker relocation payload and reflection flags
; follow surviving sideeffect calls to the final function definition.
; OBJ: "name": "pkg.marked"
; OBJ: "reflect_method"
; OBJ: "type": "R_USEIFACEMETHOD"
; OBJ: "addend": 17
; OBJ: "sym_index": 7
; OBJ: "package": "types"
; OBJ: "name": "types.marker"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-v128:64:64-v256:64:64-v512:64:64-n8:16:32:64-S64"
target triple = "x86_64-unknown-linux-goobj"
@types.marker = external global i8, !goobj.import !8
@llvm.compiler.used = appending global [1 x ptr] [ptr @types.marker], section "llvm.metadata"
declare void @llvm.sideeffect()
define goabiinternal void @pkg.marked() !goobj.symbol.index !0 {
  call void @llvm.sideeffect(), !goobj.marker_reloc !7
  call void @llvm.sideeffect(), !goobj.reflect_method !6
  ret void
}
!0 = !{i32 0, i16 1}
!goobj.config = !{!1}
!1 = !{!"marker-id", !"pkg", !"0", !"0", !"0100000000000000"}
!llvm.module.flags = !{!2}
!2 = !{i32 1, !"goobj.abi", !3}
!3 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !6}
!6 = !{}
!7 = !{ptr @types.marker, i32 24, i64 17}
!8 = !{!"types", i32 7, i32 0}
