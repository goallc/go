; Lower LLVM instrumentation to GoObj counters and a package table.
; CHECK-DAG: @"go:llvmpgocount" = internal global [2 x i64] zeroinitializer
; CHECK-DAG: @"go:llvmpgopkg.p" = constant
; CHECK-DAG: @llvm.global_ctors = appending global [1 x {{.*}}@other_init
; CHECK-LABEL: define goabi0 i64 @"p.choose<ABI0>"
; CHECK: alloca i64
; CHECK: atomicrmw add ptr {{.*}}, i64 1 monotonic
; CHECK: atomicrmw add ptr {{.*}}, i64 %{{.*}} monotonic
; CHECK-NOT: atomicrmw
; CHECK: ret i64

source_filename = "p"
target datalayout = "e-p:64:64"
target triple = "x86_64-unknown-linux-goobj"

@llvm.global_ctors = appending global [1 x {i32, ptr, ptr}] [{i32, ptr, ptr} {i32 17, ptr @other_init, ptr null}]
declare void @other_init()
declare void @use(ptr)

define goabi0 i64 @"p.choose<ABI0>"(i1 %cond, i64 %a, i64 %b) {
  %slot = alloca i64
  call void @use(ptr %slot)
  %v = select i1 %cond, i64 %a, i64 %b
  ret i64 %v
}
