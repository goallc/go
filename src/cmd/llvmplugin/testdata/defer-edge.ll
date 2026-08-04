target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @runtime.deferproc()
declare goabiinternal void @runtime.deferreturn()
declare void @llvm.go.defer.edge()

define goabiinternal void @defer_edge() #0 gc "goallc" {
entry:
  call goabiinternal void @runtime.deferproc()
  callbr void @llvm.go.defer.edge() to label %normal [label %recover]

normal:
  ret void

recover:
  call goabiinternal void @runtime.deferreturn()
  ret void
}

define goabiinternal void @defer_wrapper() !goobj.func.info !0 {
entry:
  ret void
}

attributes #0 = { "go-stack-growth-statepoint" }

!0 = !{i8 23, i8 0}
