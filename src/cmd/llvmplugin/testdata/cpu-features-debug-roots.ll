; RUN: opt -load-pass-plugin=%plugin -passes=goallc-cpu-features,globaldce -S %s | FileCheck %s
; No named function table is needed for definitions. Even an uncalled internal
; function must survive GlobalDCE when Go debug variables still use its scope.
target triple = "x86_64-unknown-linux-goobj"

; CHECK: @llvm.compiler.used = appending global [2 x ptr] [ptr @uncalled, ptr @abstract]
; CHECK: define internal void @uncalled() !dbg ![[SP:[0-9]+]]
; CHECK: !goobj.debug.funcs = !{![[ABSTRACT:[0-9]+]]}
; CHECK: ![[ABSTRACT]] = !{!{{[0-9]+}}, ptr @abstract}
define internal void @uncalled() !dbg !4 {
  ret void, !dbg !5
}
declare void @abstract()
!goallc.cpu.config = !{!0}
!llvm.dbg.cu = !{!1}
!llvm.module.flags = !{!6}
!goobj.debug.funcs = !{!7}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}
!1 = distinct !DICompileUnit(language: DW_LANG_Go, file: !2, producer: "goallc", isOptimized: true, runtimeVersion: 0, emissionKind: FullDebug)
!2 = !DIFile(filename: "roots.go", directory: "/tmp")
!3 = !DISubroutineType(types: !{})
!4 = distinct !DISubprogram(name: "uncalled", scope: !2, file: !2, line: 1, type: !3, scopeLine: 1, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !1)
!5 = !DILocation(line: 1, scope: !4)
!6 = !{i32 2, !"Debug Info Version", i32 3}
!7 = !{!8, ptr @abstract}
!8 = distinct !DISubprogram(name: "abstract", scope: !2, file: !2, line: 2, type: !3, scopeLine: 2, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !1)
