target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
%go.abi.pad = type <{ i8 }>
define goabiinternal i64 @caller.Padded(%go.abi.pad %p) !goobj.symbol.index !0 {
 ret i64 0
}
define goabiinternal ptr @caller.Closure() !goobj.symbol.index !1 {
 %p = call goabiinternal ptr @callee.GetClosure()
 ret ptr %p
}
declare !goobj.import !3 goabiinternal ptr @callee.GetClosure()
!3 = !{!"callee", i32 1, i8 0}
!goobj.imports = !{!4}
!4 = !{!"callee", !"callee", !"0100000000000000"}
!goobj.config = !{!900}
!900 = !{!"id", !"caller", !"0", !"0", !"0100000000000000"}
!901 = !{}
!0 = !{i32 0}
!1 = !{i32 1}
!2 = !{i1 true}

define goabiinternal ptr @caller.ABI0() !goobj.symbol.index !10 {
 %p = call goabiinternal ptr @callee.ABI0Getter()
 ret ptr %p
}
declare !goobj.import !11 goabiinternal ptr @callee.ABI0Getter()
!10 = !{i32 2}
!11 = !{!"callee", i32 2, i8 0}

!llvm.module.flags = !{!902}
!902 = !{i32 1, !"goobj.abi", !903}
!903 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !901}
