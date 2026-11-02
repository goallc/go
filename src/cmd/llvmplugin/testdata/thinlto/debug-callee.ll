; The source function is linknamed: its DISubprogram name is not its IR name.
; The caller must recover the exact symbol even after the body is inlined away.
target triple = "x86_64-unknown-linux-goobj"
@dep.slot = global i64 41, !goobj.symbol.index !7
define goabiinternal i64 @dep.public() !goobj.symbol.index !0 !dbg !13 {
  %v = load volatile i64, ptr @dep.slot, !dbg !14
  ret i64 %v, !dbg !14
}
!0 = !{i32 0, i16 1}
!7 = !{i32 1, i16 0}
!goobj.config = !{!2}
!2 = !{!"dep-id", !"dep", !"0", !"0", !"0200000000000000"}
!goobj.imports = !{}
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
