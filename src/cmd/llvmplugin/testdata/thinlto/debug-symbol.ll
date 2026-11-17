; An imported inline scope keeps its Go symbol identity after the source body
; and even its declaration have disappeared. The source/linkage names may be
; different from the actual symbol. ABI0 and the package index must survive.
; OBJ: "inline_tree": [
; OBJ: "sym_index": 7,
; OBJ: "package": "dep",
; OBJ: "name": "dep.actual"
; BAD: invalid GoObj subprogram package annotation

define goabiinternal i64 @caller(i64 %x) !dbg !5 {
  %r = add i64 %x, 1, !dbg !8
  ret i64 %r, !dbg !8
}

!llvm.dbg.cu = !{!0}
!llvm.module.flags = !{!3, !4}
!0 = distinct !DICompileUnit(language: DW_LANG_Go, file: !1, producer: "Go compiler", isOptimized: true, runtimeVersion: 0, emissionKind: LineTablesOnly)
!1 = !DIFile(filename: "caller.go", directory: "/tmp")
!2 = !DISubroutineType(types: !9)
!3 = !{i32 2, !"Dwarf Version", i32 4}
!4 = !{i32 2, !"Debug Info Version", i32 3}
!5 = distinct !DISubprogram(name: "caller", linkageName: "caller", scope: !1, file: !1, line: 1, type: !2, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0)
!6 = distinct !DISubprogram(name: "dep.source", linkageName: "dep.source", scope: !1, file: !1, line: 20, type: !2, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, annotations: !10)
!7 = !DILocation(line: 2, scope: !5)
!8 = !DILocation(line: 21, scope: !6, inlinedAt: !7)
!9 = !{}
!10 = !{!11, !12, !13, !14}
!11 = !{!"goobj.symbol", !"dep.actual<ABI0>"}
!12 = !{!"goobj.package", !"dep"}
!13 = !{!"goobj.index", i32 7}
!14 = !{!"goobj.flags2", i32 0}
