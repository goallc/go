; RUN: opt -load-pass-plugin=%plugin -passes=goallc-cpu-features,goallc-cpu-features -S %s | FileCheck %s
;
; The early pass clones the guarded feature path, specializes each guard, and
; leaves one public lazy dispatcher. Running it twice is deliberately part of
; the test: the module marker must make every entry idempotent.

target triple = "x86_64-unknown-linux-gnu"

@runtime.goallcCPUFeatures = external global i8

; CHECK: @round.goallc.fmv.slot = internal global ptr @round.goallc.fmv.resolve, section ".noptrdata", align 8, !goobj.symbol.nonpackage ![[NONPACKAGE:[0-9]+]]

declare double @fallback(double)
declare double @llvm.floor.f64(double)

; CHECK-LABEL: define double @round(
; CHECK-SAME: #[[EARLY_DISPATCH:[0-9]+]]
; CHECK-SAME: !dbg ![[SUBPROGRAM:[0-9]+]]
; CHECK-SAME: !goobj.symbol.index ![[SYMINDEX:[0-9]+]]
; CHECK-SAME: !goobj.symbol.flags ![[SYMFLAGS:[0-9]+]]
; CHECK-SAME: !goobj.func.info ![[FUNCINFO:[0-9]+]]
; CHECK: entry:
; CHECK-NEXT: %target = load atomic ptr, ptr @round.goallc.fmv.slot monotonic, align 8, !dbg ![[DISPATCHLOC:[0-9]+]], !goallc.cpu.dispatcher.inline ![[DISPATCHMARK:[0-9]+]]
; CHECK-NEXT: %{{.*}} = tail call double %target(double %x), !dbg ![[DISPATCHLOC]], !goallc.cpu.dispatcher.inline ![[DISPATCHMARK]]
; CHECK-NEXT: ret double

; CHECK-INLINE-LABEL: define double @round.caller(
; CHECK-INLINE: %target.i = load atomic ptr, ptr @round.goallc.fmv.slot monotonic, align 8, !dbg ![[CALLERLOC:[0-9]+]]
; CHECK-INLINE-NOT: call double @round(
; CHECK-INLINE: %{{.*}} = tail call double %target.i(double %x), !dbg ![[CALLERLOC]], !callees !{{[0-9]+}}, !inline_history !{{[0-9]+}}
; CHECK-INLINE: fadd double
; CHECK-INLINE-NOT: !goallc.cpu.dispatcher.inline
; CHECK-INLINE: ![[CALLERSP:[0-9]+]] = distinct !DISubprogram(name: "round.caller"
; CHECK-INLINE: ![[CALLERLOC]] = distinct !DILocation(line: 20, column: 1, scope: ![[CALLERSP]])

; CHECK-FINAL-LABEL: define double @round(
; CHECK-FINAL: musttail call double %target(double %x)
; CHECK-FINAL-NOT: !goallc.cpu.dispatcher.inline
; CHECK-FINAL-NOT: "goallc.cpu.tail-transfers"
; CHECK-FINAL-LABEL: define internal double @round.goallc.fmv.resolve(
; CHECK-FINAL: musttail call double @round.goallc.fmv.baseline(double %x)
; CHECK-FINAL: musttail call double {{.*}}(double %x)
; CHECK-FINAL-NOT: "goallc.cpu.tail-transfers"

; CHECK-X86-ASM-LABEL: round:
; CHECK-X86-ASM: movq round.goallc.fmv.slot(%rip), %rax
; CHECK-X86-ASM-NEXT: jmpq *%rax
; CHECK-X86-ASM-LABEL: round.goallc.fmv.resolve:
; CHECK-X86-ASM-NOT: callq
; CHECK-X86-ASM: jmpq *%rdx
; CHECK-X86-ASM-NOT: callq
; CHECK-X86-ASM: jmp round.goallc.fmv.baseline
define double @round(double %x) #0 !goobj.symbol.index !2 !goobj.symbol.flags !3 !goobj.func.info !15 !dbg !9 {
entry:
  %flag = load i8, ptr @runtime.goallcCPUFeatures, align 1, !goallc.cpu.guard !1, !dbg !10
  %enabled = icmp ne i8 %flag, 0, !dbg !10
  br i1 %enabled, label %feature, label %fallback, !dbg !10

feature:
  %rounded = call double @llvm.floor.f64(double %x), !goallc.cpu.requires !1, !dbg !10
  br label %done, !dbg !10

fallback:
  %soft = call double @fallback(double %x), !dbg !10
  br label %done, !dbg !10

done:
  %result = phi double [ %rounded, %feature ], [ %soft, %fallback ]
  ret double %result, !dbg !10
}

define double @round.caller(double %x) #1 !dbg !16 {
entry:
  %rounded = call double @round(double %x), !dbg !17
  %adjusted = fadd double %rounded, 1.000000e+00, !dbg !17
  ret double %adjusted, !dbg !17
}

; CHECK-LABEL: define internal double @round.goallc.fmv.baseline(
; CHECK-NOT: !goobj.symbol.flags
; CHECK-SAME: !goobj.func.info ![[FUNCINFO]]
; CHECK-NOT: !goobj.symbol.flags
; CHECK-SAME: !goobj.symbol.nonpackage ![[NONPACKAGE]]
; CHECK-NOT: llvm.floor
; CHECK: call double @fallback(double %x)
; CHECK: ret double

; CHECK-LABEL: define internal double @round.goallc.fmv.sse41(
; CHECK-SAME: #[[SSE41:[0-9]+]]
; CHECK-NOT: !goobj.symbol.flags
; CHECK-SAME: !goobj.func.info ![[FUNCINFO]]
; CHECK-NOT: !goobj.symbol.flags
; CHECK-SAME: !goobj.symbol.nonpackage ![[NONPACKAGE]]
; CHECK: call double @llvm.floor.f64(double %x){{.*}}!goallc.cpu.requires
; CHECK-NOT: call double @fallback
; CHECK: ret double

; CHECK-LABEL: define internal double @round.goallc.fmv.resolve(
; CHECK-SAME: #[[RESOLVER_ATTRS:[0-9]+]]
; CHECK-SAME: !goobj.func.info ![[FUNCINFO]]
; CHECK-SAME: !goobj.symbol.nonpackage ![[NONPACKAGE]]
; CHECK: load i64, ptr @runtime.goallcCPUFeatures
; CHECK: and i64 %features, 64
; CHECK: br i1
; CHECK: uninitialized:
; CHECK: musttail call double @round.goallc.fmv.baseline(double %x)
; CHECK: select:
; CHECK: and i64 %features, 4
; CHECK: select i1 {{.*}}, ptr @round.goallc.fmv.sse41, ptr @round.goallc.fmv.baseline
; CHECK: store atomic ptr {{.*}}, ptr @round.goallc.fmv.slot monotonic
; CHECK: musttail call double {{.*}}(double %x)

; CHECK-DAG: attributes #[[EARLY_DISPATCH]] = {{.*}}"go-nosplit" "goallc.cpu.tail-transfers" {{.*}}"target-cpu"="x86-64"
; CHECK-DAG: attributes #[[RESOLVER_ATTRS]] = {{.*}}noinline{{.*}}"go-nosplit"
; CHECK-DAG: attributes #[[SSE41]] = {{.*}}"target-cpu"="x86-64" {{.*}}"target-features"="+sse4.1"
; CHECK: !goobj.debug.inline.required = !{![[BASE_REQUIRED:[0-9]+]], ![[SSE_REQUIRED:[0-9]+]]}
; CHECK: !goallc.cpu.fmv.done = !{![[DONE:[0-9]+]]}
; CHECK: ![[NONPACKAGE]] = !{i1 true}
; CHECK-DAG: ![[BASE_REQUIRED]] = !{ptr @round.goallc.fmv.baseline, ![[BASE_INLINE:[0-9]+]]}
; CHECK-DAG: ![[SSE_REQUIRED]] = !{ptr @round.goallc.fmv.sse41, ![[SSE_INLINE:[0-9]+]]}
; CHECK-DAG: ![[BASE_INLINE]] = !DILocation(line: 10, column: 1, scope: ![[HELPER:[0-9]+]], inlinedAt: !{{[0-9]+}})
; CHECK-DAG: ![[SSE_INLINE]] = !DILocation(line: 10, column: 1, scope: ![[HELPER]], inlinedAt: !{{[0-9]+}})
; CHECK-DAG: ![[HELPER]] = distinct !DISubprogram(name: "helper", linkageName: "helper"
; CHECK: ![[DONE]] = !{!"goallc.cpu.v1"}
; CHECK: ![[SYMINDEX]] = !{i32 17}
; CHECK: ![[SYMFLAGS]] = !{i32 8, i32 0}
; Every executable source representation inherits its FuncID and flags.
; CHECK: ![[FUNCINFO]] = !{i8 10, i8 3}

attributes #0 = { "goallc.cpu.multiversion"="x86.sse41" "target-cpu"="x86-64" }
attributes #1 = { "no-builtins" "target-cpu"="x86-64" }

!goallc.cpu.config = !{!0}
!llvm.dbg.cu = !{!7}
!llvm.module.flags = !{!8}
!goobj.debug.funcs = !{!13}
!goobj.debug.inline.required = !{!12}
!0 = !{!"goallc.cpu.v1", !"amd64", !"v1"}
!1 = !{!"x86.sse41"}
!2 = !{i32 17}
!3 = !{i32 8, i32 0}
!4 = !DIFile(filename: "round.go", directory: "/tmp/goallc-cpu-features")
!5 = !{}
!6 = !DISubroutineType(types: !5)
!7 = distinct !DICompileUnit(language: DW_LANG_Go, file: !4, producer: "goallc-test", isOptimized: true, runtimeVersion: 0, emissionKind: LineTablesOnly, enums: !5, splitDebugInlining: true, nameTableKind: None)
!8 = !{i32 2, !"Debug Info Version", i32 3}
!9 = distinct !DISubprogram(name: "round", linkageName: "round", scope: !4, file: !4, line: 1, type: !6, scopeLine: 1, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !7, retainedNodes: !5)
!10 = !DILocation(line: 1, column: 1, scope: !9)
!11 = distinct !DISubprogram(name: "helper", linkageName: "helper", scope: !4, file: !4, line: 10, type: !6, scopeLine: 10, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !7, retainedNodes: !5)
!12 = !{ptr @round, !14}
!13 = !{!9, ptr @round}
!14 = !DILocation(line: 10, column: 1, scope: !11, inlinedAt: !10)
!15 = !{i8 10, i8 3}
!16 = distinct !DISubprogram(name: "round.caller", linkageName: "round.caller", scope: !4, file: !4, line: 20, type: !6, scopeLine: 20, spFlags: DISPFlagDefinition | DISPFlagOptimized, unit: !7, retainedNodes: !5)
!17 = !DILocation(line: 20, column: 1, scope: !16)
