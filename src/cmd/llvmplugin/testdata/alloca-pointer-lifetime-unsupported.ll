target triple = "x86_64-unknown-linux-goobj"

declare void @llvm.lifetime.start.p0(i64 immarg, ptr nocapture)
declare void @llvm.lifetime.end.p0(i64 immarg, ptr nocapture)
declare goabiinternal void @safepoint()

define goabiinternal void @pointer_alloca_with_lifetime() "go-stack-growth-statepoint" gc "goallc" {
entry:
  %slot = alloca ptr, align 8
  call void @llvm.lifetime.start.p0(i64 8, ptr %slot)
  call goabiinternal void @safepoint()
  call void @llvm.lifetime.end.p0(i64 8, ptr %slot)
  ret void
}
