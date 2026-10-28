target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
define goabiinternal i64 @target() !goobj.symbol.index !0 {
  ret i64 42
}
!0 = !{i32 7}
!goobj.config = !{!900}
!goobj.imports = !{}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"linkname-function-id", !"linkname-function", !"0", !"0", !"0", !901, !"0100000000000000"}
!901 = !{}
