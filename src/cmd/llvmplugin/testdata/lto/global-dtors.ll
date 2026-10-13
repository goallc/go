target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-n8:16:32:64-S128"
; ERROR: goallc-lto: optimization produced a definition without package ownership:
@llvm.global_dtors = appending global [1 x { i32, ptr, ptr }] [{ i32, ptr, ptr } { i32 1, ptr @destructor, ptr null }]
define void @destructor() {
  ret void
}

!goobj.config = !{!900}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"dtors-id", !"dtors", !"0", !"0", !"0", !901, !"0100000000000000"}
!901 = !{}
