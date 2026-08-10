target triple = "x86_64-unknown-linux-goobj"

define goabiinternal i64 @main.outer(i64 %x) !dbg !10 {
entry:
  %a = add i64 %x, 1, !dbg !30
  %b = mul i64 %a, 2, !dbg !30
  ret i64 %b, !dbg !30
}

define goabiinternal i64 @main.mid(i64 %x) !dbg !11 {
entry:
  ret i64 %x, !dbg !33
}

define goabiinternal i64 @main.inner(i64 %x) !dbg !12 {
entry:
  ret i64 %x, !dbg !34
}

define goabiinternal void @main.unlocated() !dbg !13 {
entry:
  ret void
}

!llvm.dbg.cu = !{!0}
!llvm.module.flags = !{!5, !6}
!goobj.debug.funcs = !{!40, !41, !42, !43}

!0 = distinct !DICompileUnit(language: DW_LANG_Go, file: !1, producer: "goallc-test", isOptimized: true, runtimeVersion: 0, emissionKind: LineTablesOnly, enums: !2, splitDebugInlining: true, nameTableKind: None)
!1 = !DIFile(filename: "outer.go", directory: "/tmp/goobj-inline")
!2 = !{}
!3 = !DISubroutineType(types: !2)
!4 = !DIFile(filename: "unlocated.go", directory: "/tmp/goobj-inline")
!5 = !{i32 7, !"Dwarf Version", i32 4}
!6 = !{i32 2, !"Debug Info Version", i32 3}

!10 = distinct !DISubprogram(name: "main.outer", linkageName: "main.outer", scope: !1, file: !1, line: 5, type: !3, scopeLine: 5, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)
!11 = distinct !DISubprogram(name: "main.mid", linkageName: "main.mid", scope: !1, file: !1, line: 15, type: !3, scopeLine: 15, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)
!12 = distinct !DISubprogram(name: "main.inner", linkageName: "main.inner", scope: !1, file: !1, line: 25, type: !3, scopeLine: 25, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)
!13 = distinct !DISubprogram(name: "main.unlocated", linkageName: "main.unlocated", scope: !4, file: !4, line: 40, type: !3, scopeLine: 40, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)

!30 = !DILocation(line: 30, column: 3, scope: !12, inlinedAt: !31)
!31 = distinct !DILocation(line: 20, column: 3, scope: !11, inlinedAt: !32)
!32 = distinct !DILocation(line: 10, column: 3, scope: !10)
!33 = !DILocation(line: 16, column: 2, scope: !11)
!34 = !DILocation(line: 26, column: 2, scope: !12)

!40 = !{!10, ptr @main.outer}
!41 = !{!11, ptr @main.mid}
!42 = !{!12, ptr @main.inner}
!43 = !{!13, ptr @main.unlocated}
