target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @runtime.deferproc()
declare goabiinternal void @runtime.deferreturn()
declare goabiinternal void @runtime.panicmem()
declare void @llvm.go.defer.edge()
declare void @llvm.lifetime.start.p0(ptr captures(none))

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

define goabiinternal ptr @defer_result(ptr %pointer) #0 gc "goallc" {
entry:
  %result = alloca ptr, align 8, !goallc.defer_result !1
  call void @llvm.lifetime.start.p0(ptr %result)
  store ptr null, ptr %result, align 8
  call goabiinternal void @runtime.deferproc()
  callbr void @llvm.go.defer.edge() to label %panic [label %recover]

panic:
  store ptr %pointer, ptr %result, align 8
  call goabiinternal void @runtime.panicmem()
  unreachable

recover:
  call goabiinternal void @runtime.deferreturn()
  %value = load volatile ptr, ptr %result, align 8
  ret ptr %value
}

define goabiinternal void @defer_wrapper() !goobj.func.info !0 {
entry:
  ret void
}

attributes #0 = { "go-stack-growth-statepoint" }

!0 = !{i8 23, i8 0}
!1 = !{}
