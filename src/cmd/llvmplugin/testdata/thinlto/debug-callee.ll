; CGO: "text_header": {{.*}}-lthin_callee
; CGO-NOT: -lthin_caller
; ASM-NOT: -lthin_caller
; ASM: module asm
; ASM: -lthin_callee
; ASM-NOT: -lthin_caller
; The source function is linknamed: its DISubprogram name is not its IR name.
; The caller must recover the exact symbol even after the body is inlined away.
target triple = "x86_64-unknown-linux-goobj"
@dep.slot = global i64 41, !goobj.symbol.index !7
define goabiinternal i64 @dep.public() !goobj.symbol.index !0 !dbg !13 {
  %v = load volatile i64, ptr @dep.slot, !dbg !20
  ret i64 %v, !dbg !14
}
!0 = !{i32 0, i16 1}
!7 = !{i32 1, i16 0}
!goobj.config = !{!2}
!2 = !{!"dep-id", !"dep", !"0", !"0", !"0200000000000000"}
!goobj.imports = !{!25}
!llvm.module.flags = !{!4, !15, !16}
!4 = !{i32 1, !"goobj.abi", !5}
!5 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !6}
!6 = !{}
!llvm.dbg.cu = !{!10}
!10 = distinct !DICompileUnit(language: DW_LANG_Go, file: !11, producer: "goallc-test", isOptimized: true, runtimeVersion: 0, emissionKind: LineTablesOnly)
!11 = !DIFile(filename: "dep.go", directory: "/tmp/thinlto")
!12 = !DISubroutineType(types: !6)
!13 = distinct !DISubprogram(name: "dep.source", linkageName: "dep.source", scope: !11, file: !11, line: 19, type: !12, scopeLine: 19, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !10)
!14 = !DILocation(line: 20, scope: !13)
!15 = !{i32 7, !"Dwarf Version", i32 4}
!16 = !{i32 2, !"Debug Info Version", i32 3}

; A compiler-inlined function has no body here. Its exact symbol mapping must
; survive another round of inlining, while an unused mapping must not become a
; debug root merely because its declaration survives importing.
declare !goobj.import !21 goabiinternal i64 @leaf.original()
declare !goobj.import !22 goabiinternal i64 @leaf.unused()
!17 = distinct !DISubprogram(name: "leaf.source", linkageName: "leaf.source", scope: !11, file: !11, line: 30, type: !12, scopeLine: 30, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !10)
!18 = distinct !DISubprogram(name: "leaf.unused.source", linkageName: "leaf.unused.source", scope: !11, file: !11, line: 40, type: !12, scopeLine: 40, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !10)
!20 = !DILocation(line: 31, scope: !17, inlinedAt: !14)
!21 = !{!"leaf", i32 0, i32 0}
!22 = !{!"leaf", i32 1, i32 0}
!goobj.debug.funcs = !{!23, !24}
!23 = !{!17, ptr @leaf.original}
!24 = !{!18, ptr @leaf.unused}
!25 = !{!"leaf", !"leaf", !"0300000000000000"}

module asm ".goobj.cgo \22[[\5C\22cgo_ldflag\5C\22,\5C\22-lthin_callee\5C\22]]\22\0A"
