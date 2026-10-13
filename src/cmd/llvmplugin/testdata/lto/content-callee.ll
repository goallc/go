target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
; CHECK: @callee.text = weak{{.*}} constant [3 x i8] c"abc", {{.*}}!goobj.content_hash
; CHECK-LABEL: define goabiinternal {{.*}}ptr @callee.String()
; CHECK: ret ptr @callee.text
; CHECK-LABEL: define goabiinternal void @callee.Defer()
; CHECK: callbr void @llvm.go.defer.edge()
; CHECK: declare goabi0 void @"callee.Assembly<ABI0>"()
; CHECK: !goobj.marker_relocs = !{![[MARK:[0-9]+]]}
; CHECK: ![[MARK]] = !{ptr @callee.String, ptr @"callee.Assembly<ABI0>", i32 23, i64 0}
@callee.text = weak constant [3 x i8] c"abc", !goobj.content_hash !1
define goabiinternal ptr @callee.String() !goobj.symbol.index !0 {
 call void @llvm.sideeffect(), !goobj.marker_reloc !10
 ret ptr @callee.text
}
define goabiinternal void @callee.Defer() !goobj.symbol.index !2 {
 callbr void @llvm.go.defer.edge() to label %normal [label %recover]
normal:
 ret void
recover:
 call goabi0 void @"callee.Assembly<ABI0>"()
 ret void
}
declare void @llvm.go.defer.edge()
declare goabi0 void @"callee.Assembly<ABI0>"()
!1 = !{!"0123456789abcdef"}
!2 = !{i32 1}
!goobj.marker_relocs = !{!3}
!3 = !{ptr @callee.String, ptr @"callee.Assembly<ABI0>", i32 23, i64 0}
!goobj.debug.funcs = !{!4}
!4 = !{!5, ptr @"callee.Assembly<ABI0>"}
!5 = distinct !DISubprogram(name: "callee.Assembly", linkageName: "callee.Assembly", scope: null, file: !6, line: 1, type: !7, scopeLine: 1, spFlags: DISPFlagDefinition, unit: !8)
!6 = !DIFile(filename: "callee.go", directory: "/")
!7 = !DISubroutineType(types: !901)
!8 = distinct !DICompileUnit(language: DW_LANG_Go, file: !6, producer: "GoALLC", isOptimized: true, runtimeVersion: 0, emissionKind: FullDebug)
!llvm.dbg.cu = !{!8}
!llvm.module.flags = !{!9}
!9 = !{i32 2, !"Debug Info Version", i32 3}
!goobj.config = !{!900}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"id", !"callee", !"0", !"0", !"0", !901, !"0100000000000000"}
!901 = !{}
!0 = !{i32 0}

declare void @llvm.sideeffect()
!10 = !{ptr @"callee.Assembly<ABI0>", i32 23, i64 0}
