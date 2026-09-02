target triple = "x86_64-unknown-linux-goobj"

; DEBUG-LABEL:      "name": "main.outer",
; DEBUG:            "start_line": 5,
; DEBUG:            "inline_tree": [
; DEBUG:            "parent": -1,
; DEBUG:            "line": 10,
; DEBUG:            "name": "main.mid",
; DEBUG-X86:        "parent_pc": 0
; DEBUG-AARCH64:    "parent_pc": 8
; DEBUG:            "parent": 0,
; DEBUG:            "line": 20,
; DEBUG:            "name": "main.inner",
; DEBUG-X86:        "parent_pc": 7
; DEBUG-AARCH64:    "parent_pc": 16
; DEBUG-X86:        "pc_quantum": 1,
; DEBUG-AARCH64:    "pc_quantum": 4,
; DEBUG:            "kind": "pcfile",
; DEBUG:            "value": 0,
; DEBUG:            "file": "/tmp/goobj-inline/outer.go"
; DEBUG:            "kind": "pcline",
; DEBUG:            "value": 10
; DEBUG:            "value": 20
; DEBUG:            "value": 30
; DEBUG:            "kind": "pcinline",
; DEBUG:            "value": -1
; DEBUG:            "value": 0
; DEBUG:            "value": 1

; DEBUG-LABEL:      "name": "main.unlocated",
; DEBUG:            "start_line": 40,
; DEBUG:            "files": [
; DEBUG:            "name": "/tmp/goobj-inline/unlocated.go"
; DEBUG-X86:        "pc_quantum": 1,
; DEBUG-AARCH64:    "pc_quantum": 4,
; DEBUG:            "kind": "pcfile",
; DEBUG:            "start": 0,
; DEBUG:            "value": 1,
; DEBUG:            "file": "/tmp/goobj-inline/unlocated.go"
; DEBUG:            "kind": "pcline",
; DEBUG:            "start": 0,
; DEBUG:            "value": 40

; DEBUG-LABEL:      "name": "main.zero",
; DEBUG:            "start_line": 50,
; DEBUG:            "inline_tree": [
; DEBUG:            "parent": -1,
; DEBUG:            "line": 0,
; DEBUG:            "name": "main.zeroCallee",
; DEBUG-X86:        "parent_pc": 0
; DEBUG-AARCH64:    "parent_pc": 0

; DEBUG-LABEL:      "name": "main.shared",
; DEBUG:            "start_line": 70,
; DEBUG:            "inline_tree": [
; DEBUG:            "parent": -1,
; DEBUG:            "line": 71,
; DEBUG:            "name": "main.sharedLeft",
; DEBUG-X86:        "parent_pc": 0
; DEBUG-AARCH64:    "parent_pc": 0
; DEBUG:            "parent": -1,
; DEBUG:            "line": 71,
; DEBUG:            "name": "main.sharedRight",
; DEBUG-X86:        "parent_pc": 8
; DEBUG-AARCH64:    "parent_pc": 16

; DEBUG-LABEL:      "name": "main.coalesced",
; DEBUG:            "start_line": 130,
; DEBUG:            "inline_tree": [
; DEBUG:            "parent": -1,
; DEBUG:            "line": 132,
; DEBUG:            "name": "main.coalescedMid",
; DEBUG:            "parent": 0,
; DEBUG:            "line": 126,
; DEBUG:            "name": "main.coalescedLeaf",
; DEBUG:            "parent": -1,
; DEBUG:            "line": 131,
; DEBUG:            "name": "main.coalescedLeaf",

; DEBUG-LABEL:      "name": "main.entry",
; DEBUG:            "start_line": 140,
; DEBUG:            "inline_tree": [
; DEBUG:            "parent": -1,
; DEBUG:            "line": 141,
; DEBUG:            "name": "main.entryChild",
; DEBUG-X86:        "parent_pc": 0
; DEBUG-AARCH64:    "parent_pc": 0

; DEBUG-LABEL:      "name": "main.erased",
; DEBUG:            "start_line": 100,
; DEBUG:            "inline_tree": null
; DEBUG:            "pc_quantum":

@main.sink = global i64 0

define goabiinternal i64 @main.outer(i64 %x) !dbg !10 {
entry:
  %root = load volatile i64, ptr @main.sink, !dbg !38
  #dbg_label(!68, !69)
  ; A deeper non-faulting instruction may be scheduled before its own label.
  ; The outer anchor must still select its direct child rather than the whole
  ; descendant chain carried by this instruction.
  %early = load volatile i64, ptr @main.sink, !dbg !30
  #dbg_label(!70, !71)
  %a = add i64 %root, %early, !dbg !30
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
  #dbg_label(!72, !73)
  %a = add i64 %x, 1, !dbg !35
  ret i64 %a, !dbg !35
}

define goabiinternal i64 @main.zeroCallee(i64 %x) !dbg !15 {
entry:
  ret i64 %x, !dbg !37
}

define goabiinternal i64 @main.shared(i64 %x) !dbg !16 {
entry:
  #dbg_label(!74, !75)
  store volatile i64 %x, ptr @main.sink, !dbg !50
  #dbg_label(!76, !77)
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

; Model two instances of the same inline callee whose equivalent operations
; were coalesced into one instruction. LLVM retains one inlinedAt chain on the
; instruction and both standard debug labels.
define goabiinternal void @main.coalesced(i64 %x) !dbg !22 {
entry:
  #dbg_label(!86, !87)
  #dbg_label(!88, !89)
  #dbg_label(!90, !91)
  store volatile i64 %x, ptr @main.sink, !dbg !81
  %mid = load volatile i64, ptr @main.sink, !dbg !95
  ret void, !dbg !92
}

define goabiinternal void @main.coalescedMid() !dbg !23 {
entry:
  ret void, !dbg !94
}

define goabiinternal void @main.coalescedLeaf() !dbg !24 {
entry:
  ret void, !dbg !93
}

; Scheduling may move a child instruction before its preserved inline label.
; Keep a concrete parent PC at function byte zero so a function value still
; resolves to the outer function.
define goabiinternal i64 @main.entry(i64 %x) !dbg !25 {
entry:
  %early = load volatile i64, ptr @main.sink, !dbg !96
  #dbg_label(!97, !98)
  %late = add i64 %early, %x, !dbg !96
  ret i64 %late, !dbg !99
}

define goabiinternal void @main.entryChild() !dbg !26 {
entry:
  ret void, !dbg !101
}

; Model inline bodies whose instructions disappeared after the frontend emitted
; their standard debug labels. The labels remain useful history, but must not
; manufacture active Go inline frames or machine instructions.
define goabiinternal void @main.erased() !dbg !19 {
entry:
  #dbg_label(!78, !79)
  #dbg_label(!80, !60)
  store volatile i64 0, ptr @main.sink, !dbg !65
  ret void, !dbg !65
}

define goabiinternal void @main.erasedMid() !dbg !20 {
entry:
  ret void, !dbg !66
}

define goabiinternal void @main.erasedInner() !dbg !21 {
entry:
  ret void, !dbg !67
}

!llvm.dbg.cu = !{!0}
!llvm.module.flags = !{!5, !6}
!goobj.debug.funcs = !{!40, !41, !42, !43, !44, !45, !46, !47, !48, !49, !56, !57, !58, !59, !63, !64, !102}

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
!19 = distinct !DISubprogram(name: "main.erased", linkageName: "main.erased", scope: !1, file: !1, line: 100, type: !3, scopeLine: 100, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)
!20 = distinct !DISubprogram(name: "main.erasedMid", linkageName: "main.erasedMid", scope: !1, file: !1, line: 110, type: !3, scopeLine: 110, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)
!21 = distinct !DISubprogram(name: "main.erasedInner", linkageName: "main.erasedInner", scope: !1, file: !1, line: 120, type: !3, scopeLine: 120, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)
!22 = distinct !DISubprogram(name: "main.coalesced", linkageName: "main.coalesced", scope: !1, file: !1, line: 130, type: !3, scopeLine: 130, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)
!23 = distinct !DISubprogram(name: "main.coalescedMid", linkageName: "main.coalescedMid", scope: !1, file: !1, line: 125, type: !3, scopeLine: 125, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)
!24 = distinct !DISubprogram(name: "main.coalescedLeaf", linkageName: "main.coalescedLeaf", scope: !1, file: !1, line: 120, type: !3, scopeLine: 120, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)
!25 = distinct !DISubprogram(name: "main.entry", linkageName: "main.entry", scope: !1, file: !1, line: 140, type: !3, scopeLine: 140, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)
!26 = distinct !DISubprogram(name: "main.entryChild", linkageName: "main.entryChild", scope: !1, file: !1, line: 150, type: !3, scopeLine: 150, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !0, retainedNodes: !2)

!30 = !DILocation(line: 30, column: 3, scope: !12, inlinedAt: !31)
!31 = distinct !DILocation(line: 20, column: 3, scope: !11, inlinedAt: !32)
!32 = distinct !DILocation(line: 10, column: 3, scope: !10)
!33 = !DILocation(line: 16, column: 2, scope: !11)
!34 = !DILocation(line: 26, column: 2, scope: !12)
!35 = !DILocation(line: 61, column: 2, scope: !15, inlinedAt: !36)
!36 = distinct !DILocation(line: 0, column: 0, scope: !14)
!37 = !DILocation(line: 61, column: 2, scope: !15)
!38 = !DILocation(line: 10, column: 7, scope: !10)
!50 = !DILocation(line: 81, column: 2, scope: !17, inlinedAt: !52)
!51 = !DILocation(line: 91, column: 2, scope: !18, inlinedAt: !52)
!52 = distinct !DILocation(line: 71, column: 2, scope: !16)
!53 = !DILocation(line: 72, column: 2, scope: !16)
!54 = !DILocation(line: 81, column: 2, scope: !17)
!55 = !DILocation(line: 91, column: 2, scope: !18)
!60 = !DILocation(line: 121, column: 2, scope: !21, inlinedAt: !61)
!61 = distinct !DILocation(line: 111, column: 2, scope: !20, inlinedAt: !62)
!62 = distinct !DILocation(line: 101, column: 2, scope: !19)
!65 = !DILocation(line: 102, column: 2, scope: !19)
!66 = !DILocation(line: 112, column: 2, scope: !20)
!67 = !DILocation(line: 122, column: 2, scope: !21)
!68 = !DILabel(scope: !11, name: "$go.inlmark.0", file: !1, line: 10, isArtificial: true)
!69 = !DILocation(line: 10, column: 3, scope: !11, inlinedAt: !32)
!70 = !DILabel(scope: !12, name: "$go.inlmark.1", file: !1, line: 20, isArtificial: true)
!71 = !DILocation(line: 20, column: 3, scope: !12, inlinedAt: !31)
!72 = !DILabel(scope: !15, name: "$go.inlmark.2", file: !1, line: 0, isArtificial: true)
!73 = !DILocation(line: 0, column: 0, scope: !15, inlinedAt: !36)
!74 = !DILabel(scope: !17, name: "$go.inlmark.3", file: !1, line: 71, isArtificial: true)
!75 = !DILocation(line: 71, column: 2, scope: !17, inlinedAt: !52)
!76 = !DILabel(scope: !18, name: "$go.inlmark.4", file: !1, line: 71, isArtificial: true)
!77 = !DILocation(line: 71, column: 2, scope: !18, inlinedAt: !52)
!78 = !DILabel(scope: !20, name: "$go.inlmark.5", file: !1, line: 101, isArtificial: true)
!79 = !DILocation(line: 101, column: 2, scope: !20, inlinedAt: !62)
!80 = !DILabel(scope: !21, name: "$go.inlmark.6", file: !1, line: 111, isArtificial: true)
!81 = !DILocation(line: 121, column: 2, scope: !24, inlinedAt: !82)
!82 = distinct !DILocation(line: 131, column: 2, scope: !22)
!84 = distinct !DILocation(line: 126, column: 2, scope: !23, inlinedAt: !85)
!85 = distinct !DILocation(line: 132, column: 2, scope: !22)
!86 = !DILabel(scope: !24, name: "$go.inlmark.7", file: !1, line: 131, isArtificial: true)
!87 = !DILocation(line: 131, column: 2, scope: !24, inlinedAt: !82)
!88 = !DILabel(scope: !23, name: "$go.inlmark.8", file: !1, line: 132, isArtificial: true)
!89 = !DILocation(line: 132, column: 2, scope: !23, inlinedAt: !85)
!90 = !DILabel(scope: !24, name: "$go.inlmark.9", file: !1, line: 126, isArtificial: true)
!91 = !DILocation(line: 126, column: 2, scope: !24, inlinedAt: !84)
!92 = !DILocation(line: 133, column: 1, scope: !22)
!93 = !DILocation(line: 121, column: 1, scope: !24)
!94 = !DILocation(line: 126, column: 1, scope: !23)
!95 = !DILocation(line: 127, column: 2, scope: !23, inlinedAt: !85)
!96 = !DILocation(line: 151, column: 2, scope: !26, inlinedAt: !100)
!97 = !DILabel(scope: !26, name: "$go.inlmark.10", file: !1, line: 141, isArtificial: true)
!98 = !DILocation(line: 141, column: 2, scope: !26, inlinedAt: !100)
!99 = !DILocation(line: 142, column: 1, scope: !25)
!100 = distinct !DILocation(line: 141, column: 2, scope: !25)
!101 = !DILocation(line: 151, column: 1, scope: !26)

!40 = !{!10, ptr @main.outer}
!41 = !{!11, ptr @main.mid}
!42 = !{!12, ptr @main.inner}
!43 = !{!13, ptr @main.unlocated}
!44 = !{!14, ptr @main.zero}
!45 = !{!15, ptr @main.zeroCallee}
!46 = !{!16, ptr @main.shared}
!47 = !{!17, ptr @main.sharedLeft}
!48 = !{!18, ptr @main.sharedRight}
!49 = !{!19, ptr @main.erased}
!56 = !{!20, ptr @main.erasedMid}
!57 = !{!21, ptr @main.erasedInner}
!58 = !{!22, ptr @main.coalesced}
!59 = !{!23, ptr @main.coalescedMid}
!63 = !{!24, ptr @main.coalescedLeaf}
!64 = !{!25, ptr @main.entry}
!102 = !{!26, ptr @main.entryChild}
