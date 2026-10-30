target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
%callee.Byte = type { i8 }
define goabiinternal i8 @callee.Byte(%callee.Byte %p) !goobj.symbol.index !0 {
 %v = extractvalue %callee.Byte %p, 0
 ret i8 %v
}
define goabiinternal ptr @callee.GetClosure() !goobj.symbol.index !1 {
 ret ptr @callee.Closure
}
define goabiinternal i64 @callee.Closure(i64 %n) noinline !goobj.content_addressable !2 {
 ret i64 %n
}
!goobj.config = !{!900}
!900 = !{!"id", !"callee", !"0", !"0", !"0100000000000000"}
!901 = !{}
!0 = !{i32 0}
!1 = !{i32 1}
!2 = !{i1 true}

; ABI0-LABEL: define goabi0 void @"callee.ABI0<ABI0>"()
; ABI0-SAME: !goobj.symbol.index ![[IDX:[0-9]+]]
; ABI0: ![[IDX]] = !{i32 4, i16 0}
define goabiinternal ptr @callee.ABI0Getter() !goobj.symbol.index !10 {
 ret ptr @"callee.ABI0<ABI0>"
}
define goabi0 void @"callee.ABI0<ABI0>"() noinline !goobj.content_addressable !2 {
 ret void
}
!10 = !{i32 2}

!llvm.module.flags = !{!902}
!902 = !{i32 1, !"goobj.abi", !903}
!903 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !901}
