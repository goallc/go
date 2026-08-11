target triple = "x86_64-unknown-linux-goobj"

@main.sink = global i64 0

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

define goabiinternal i64 @main.zero(i64 %x) !dbg !14 {
entry:
  %a = add i64 %x, 1, !dbg !35
  ret i64 %a, !dbg !35
}

define goabiinternal i64 @main.zeroCallee(i64 %x) !dbg !15 {
entry:
  ret i64 %x, !dbg !37
}

define goabiinternal i64 @main.shared(i64 %x) !dbg !16 {
entry:
  store volatile i64 %x, ptr @main.sink, !dbg !50
  store volatile i64 %x, ptr @main.sink, !dbg !51
  ret i64 %x, !dbg !53
}

define goabiinternal i64 @main.sharedLeft(i64 %x) !dbg !17 {
entry:
  ret i64 %x, !dbg !54
}

define goabiinternal i64 @main.sharedRight(i64 %x) !dbg !18 {
entry:
  ret i64 %x, !dbg !55
}

!llvm.dbg.cu = !{!0}
!llvm.module.flags = !{!5, !6}
!goobj.debug.funcs = !{!40, !41, !42, !43, !44, !45, !46, !47, !48}

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
!14 = distinct !DISubprogram(name: "main.zero", linkageName: "main.zero", scope: !1, file: !1, line: 50, type: !3, scopeLine: 50, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)
!15 = distinct !DISubprogram(name: "main.zeroCallee", linkageName: "main.zeroCallee", scope: !1, file: !1, line: 60, type: !3, scopeLine: 60, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)
!16 = distinct !DISubprogram(name: "main.shared", linkageName: "main.shared", scope: !1, file: !1, line: 70, type: !3, scopeLine: 70, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)
!17 = distinct !DISubprogram(name: "main.sharedLeft", linkageName: "main.sharedLeft", scope: !1, file: !1, line: 80, type: !3, scopeLine: 80, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)
!18 = distinct !DISubprogram(name: "main.sharedRight", linkageName: "main.sharedRight", scope: !1, file: !1, line: 90, type: !3, scopeLine: 90, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)

!30 = !DILocation(line: 30, column: 3, scope: !12, inlinedAt: !31)
!31 = distinct !DILocation(line: 20, column: 3, scope: !11, inlinedAt: !32)
!32 = distinct !DILocation(line: 10, column: 3, scope: !10)
!33 = !DILocation(line: 16, column: 2, scope: !11)
!34 = !DILocation(line: 26, column: 2, scope: !12)
!35 = !DILocation(line: 61, column: 2, scope: !15, inlinedAt: !36)
!36 = distinct !DILocation(line: 0, column: 0, scope: !14)
!37 = !DILocation(line: 61, column: 2, scope: !15)
!50 = !DILocation(line: 81, column: 2, scope: !17, inlinedAt: !52)
!51 = !DILocation(line: 91, column: 2, scope: !18, inlinedAt: !52)
!52 = distinct !DILocation(line: 71, column: 2, scope: !16)
!53 = !DILocation(line: 72, column: 2, scope: !16)
!54 = !DILocation(line: 81, column: 2, scope: !17)
!55 = !DILocation(line: 91, column: 2, scope: !18)

!40 = !{!10, ptr @main.outer}
!41 = !{!11, ptr @main.mid}
!42 = !{!12, ptr @main.inner}
!43 = !{!13, ptr @main.unlocated}
!44 = !{!14, ptr @main.zero}
!45 = !{!15, ptr @main.zeroCallee}
!46 = !{!16, ptr @main.shared}
!47 = !{!17, ptr @main.sharedLeft}
!48 = !{!18, ptr @main.sharedRight}
