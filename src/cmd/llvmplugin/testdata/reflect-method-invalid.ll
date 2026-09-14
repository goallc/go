target triple = "x86_64-unknown-linux-goobj"

; CHECK: GoALLC reflection method marker metadata must be empty

declare void @llvm.sideeffect()
define goabiinternal void @invalid() "go-stack-growth-statepoint" gc "goallc" {
  call void @llvm.sideeffect(), !goobj.reflect_method !0
  ret void
}
!0 = !{i32 1}
