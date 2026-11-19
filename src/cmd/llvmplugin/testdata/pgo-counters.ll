; The same early pipeline runs again before codegen. Each select and CFG
; counter must be lowered once and refer to the published package table.
; CHECK: @"go:llvmpgocount" = internal global [2 x i64] zeroinitializer
; CHECK: @"go:llvmpgopkg.p" = constant
; CHECK-LABEL: define goabi0 i64 @"p.choose<ABI0>"
; CHECK: alloca i64
; CHECK: atomicrmw add ptr {{.*}}, i64 1 monotonic
; CHECK: atomicrmw add ptr {{.*}}, i64 %{{.*}} monotonic
; CHECK-NOT: atomicrmw
; CHECK: ret i64

source_filename = "p"
target datalayout = "e-p:64:64"
target triple = "x86_64-unknown-linux-goobj"

declare void @use(ptr)

define goabi0 i64 @"p.choose<ABI0>"(i1 %cond, i64 %a, i64 %b) {
  %slot = alloca i64
  call void @use(ptr %slot)
  %v = select i1 %cond, i64 %a, i64 %b
  ret i64 %v
}

!goallc.pgo = !{!0}
!0 = !{!"generate"}
