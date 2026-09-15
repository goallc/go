target triple = "x86_64-unknown-linux-goobj"
declare void @goallc.gc.write.record(ptr, ptr, i32)
define void @invalid(ptr %dst, ptr %value, i32 %flags) gc "goallc" {
  call void @goallc.gc.write.record(ptr %value, ptr %dst, i32 %flags)
  store ptr %value, ptr %dst
  ret void
}
; CHECK: invalid Go write barrier record flags
