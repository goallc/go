; ERROR: records overlap

target triple = "x86_64-unknown-linux-goobj"
declare goabiinternal void @callee()
declare token @llvm.experimental.gc.statepoint.p0(i64 immarg, i32 immarg, ptr, i32 immarg, i32 immarg, ...)
define goabiinternal void @test() gc "goallc" {
entry:
  %slot = alloca [2 x ptr], align 8
  store [2 x ptr] zeroinitializer, ptr %slot, align 8
  %statepoint = call goabiinternal token (i64, i32, ptr, i32, i32, ...) @llvm.experimental.gc.statepoint.p0(i64 1, i32 0, ptr elementtype(void ()) @callee, i32 0, i32 0, i32 0, i32 0) [ "deopt"(i64 1195461697, i64 28, i64 2, i64 1095520067, i64 12, ptr %slot, i64 0, i64 8, i64 8, i64 8, i64 1, i64 1, i64 64, i64 1, i64 1, i64 1095520067, i64 12, ptr %slot, i64 0, i64 16, i64 8, i64 8, i64 1, i64 2, i64 64, i64 1, i64 3, i64 1095519299, i64 28), "gc-live"(ptr %slot) ]
  ret void
}
