target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
define goabiinternal i64 @dep.Add(i64 %x) gc "goallc" !dbg !10 !goobj.symbol.index !0 {
  #dbg_value(i64 %x, !14, !DIExpression(), !16)
  %result = add i64 %x, 7, !dbg !16
  #dbg_value(i64 %result, !15, !DIExpression(), !16)
  ret i64 %result, !dbg !16
}
!0 = !{i32 0}
!goobj.config = !{!1}
!1 = !{!"dep-id", !"dep", !"0", !"0", !"0100000000000000"}
!llvm.module.flags = !{!2, !4, !5}
!2 = !{i32 1, !"goobj.abi", !3}
!3 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !6}
!4 = !{i32 2, !"Dwarf Version", i32 4}
!5 = !{i32 2, !"Debug Info Version", i32 3}
!6 = !{}
!llvm.dbg.cu = !{!7}
!7 = distinct !DICompileUnit(language: DW_LANG_Go, file: !8, producer: "test", isOptimized: true, runtimeVersion: 0, emissionKind: FullDebug, imports: !18)
!8 = !DIFile(filename: "dep", directory: "")
!9 = !DIFile(filename: "dep.go", directory: "/src")
!10 = distinct !DISubprogram(name: "dep.Add", linkageName: "dep.Add", scope: !20, file: !9, line: 1, type: !11, scopeLine: 1, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !7, retainedNodes: !17)
!11 = !DISubroutineType(types: !12)
!12 = !{!13, !13}
!13 = !DIBasicType(name: "int", size: 64, encoding: DW_ATE_signed)
!14 = !DILocalVariable(name: "x", arg: 1, scope: !10, file: !9, line: 1, type: !13)
!15 = !DILocalVariable(name: "result", arg: 2, scope: !10, file: !9, line: 1, type: !13, annotations: !21)
!16 = !DILocation(line: 1, column: 1, scope: !10)
!17 = !{!14, !15}
!18 = !{!19}
!19 = !DIImportedEntity(tag: DW_TAG_imported_module, scope: !7, entity: !20, file: !8)
!20 = !DIModule(scope: null, name: "declareddep", file: !8)
!21 = !{!22}
!22 = !{!"go.return", !"1"}
