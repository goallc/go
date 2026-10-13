target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
; CHECK: @callee.text = weak{{.*}} constant [3 x i8] c"abc", {{.*}}!goobj.content_hash
; CHECK-LABEL: define goabiinternal {{.*}}ptr @caller.String()
; CHECK-NOT: call goabiinternal
; CHECK: call void @llvm.sideeffect(), !goobj.marker_reloc ![[INLINE:[0-9]+]]
; CHECK: ret ptr @callee.text
; CHECK: !goobj.debug.funcs = !{![[FUNC:[0-9]+]]}
; CHECK: ![[FUNC]] = !{![[SP:[0-9]+]], ptr @"callee.Assembly<ABI0>"}
; CHECK: ![[SP]] = distinct !DISubprogram(
define goabiinternal ptr @caller.String() !goobj.symbol.index !0 {
 %p = call goabiinternal ptr @callee.String()
 ret ptr %p
}
declare !goobj.import !1 goabiinternal ptr @callee.String()
!1 = !{!"callee", i32 0, i8 0}
!goobj.imports = !{!2}
!2 = !{!"callee", !"callee", !"0100000000000000"}
!goobj.config = !{!900}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"id", !"caller", !"0", !"0", !"0", !901, !"0100000000000000"}
!901 = !{}
!0 = !{i32 0}
; CHECK: ![[INLINE]] = !{ptr @"callee.Assembly<ABI0>", i32 23, i64 0}
