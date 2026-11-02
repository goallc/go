; A weak duplicate must not silently lose its body to the same storage name
; in content-main.ll. Input producers must distinguish content-addressed copies.
; ERROR: duplicate content-addressable function identity: dep.Closure#AQAAAAAAAAA=#
target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
define weak goabiinternal i64 @"dep.Closure#AQAAAAAAAAA=#"() !goobj.content_addressable !1 {
 ret i64 99
}
!goobj.config = !{!0}
!0 = !{!"id", !"collision", !"0", !"0", !"0100000000000000"}
!1 = !{i1 true}
!3 = !{}
!llvm.module.flags = !{!4}
!4 = !{i32 1, !"goobj.abi", !5}
!5 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !3}
