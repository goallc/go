; OWNER: "text_header": {{.*}}main-id
; OWNER: "fingerprint": "0100000000000000"
; CGO: "text_header": {{.*}}-lthin_caller
; CGO-NOT: -lthin_callee
; ASM-NOT: -lthin_callee
; ASM: module asm
; ASM: -lthin_caller
; ASM-NOT: -lthin_callee
; BUFFER: object 0: go object
; BUFFER-NEXT: object 1: go object
; CHECK-LABEL: define {{.*}}@main.result(
; CHECK-NOT: call {{.*}}@dep.public
; CHECK: load volatile i64, ptr @dep.slot
; CHECK-NOT: !goobj.debug.funcs
; CHECK-NOT: !goallc.thin.debug.funcs
; CHECK-DAG: distinct !DISubprogram(name: "dep.source", linkageName: "dep.source", {{.*}}annotations:
; CHECK-DAG: distinct !DISubprogram(name: "leaf.source", linkageName: "leaf.source", {{.*}}annotations:
; CHECK-DAG: !{!"goobj.symbol", !"dep.public"}
; CHECK-DAG: !{!"goobj.symbol", !"leaf.original"}
; CHECK-DAG: !{!"goobj.package", !"leaf"}
; CHECK-DAG: !DILocation(line: 20, {{.*}}inlinedAt:
; OBJ-LABEL: "name": "main.result",
; OBJ: "inline_tree": [
; OBJ: "name": "dep.public"
; OBJ: "file": "/tmp/thinlto/dep.go"
; OBJ: "name": "leaf.original"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-v128:64:64-v256:64:64-v512:64:64-n8:16:32:64-S64"
target triple = "x86_64-unknown-linux-goobj"
define goabiinternal i64 @main.result() !goobj.symbol.index !0 !dbg !13 {
  %v = call goabiinternal i64 @dep.public(), !dbg !14
  ret i64 %v, !dbg !14
}
declare !goobj.import !1 goabiinternal i64 @dep.public()
!0 = !{i32 0, i16 1}
!1 = !{!"dep", i32 0, i32 0}
!goobj.config = !{!2}
!2 = !{!"main-id", !"main", !"1", !"0", !"0100000000000000"}
!goobj.imports = !{!3}
!3 = !{!"dep", !"dep", !"0200000000000000"}
!llvm.module.flags = !{!4, !15, !16}
!4 = !{i32 1, !"goobj.abi", !5}
!5 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !6}
!6 = !{}
!llvm.dbg.cu = !{!10}
!10 = distinct !DICompileUnit(language: DW_LANG_Go, file: !11, producer: "goallc-test", isOptimized: true, runtimeVersion: 0, emissionKind: LineTablesOnly)
!11 = !DIFile(filename: "main.go", directory: "/tmp/thinlto")
!12 = !DISubroutineType(types: !6)
!13 = distinct !DISubprogram(name: "main.result", linkageName: "main.result", scope: !11, file: !11, line: 1, type: !12, scopeLine: 1, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !10)
!14 = !DILocation(line: 2, scope: !13)
!15 = !{i32 7, !"Dwarf Version", i32 4}
!16 = !{i32 2, !"Debug Info Version", i32 3}

module asm ".goobj.cgo \22[[\5C\22cgo_ldflag\5C\22,\5C\22-lthin_caller\5C\22]]\22\0A"
