target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
; ERROR: goallc-lto: duplicate symbol: caller.Answer
define goabiinternal i64 @caller.Answer() !goobj.symbol.index !0 {
  ret i64 0
}
!0 = !{i32 0}
!goobj.config = !{!900}
!goobj.imports = !{}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"duplicate-symbol-id", !"duplicate-symbol", !"0", !"0", !"0", !901, !"0100000000000000"}
!901 = !{}
