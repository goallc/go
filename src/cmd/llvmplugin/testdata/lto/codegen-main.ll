target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
; IR-LABEL: define dso_local goabiinternal i64 @main.Live(
; IR-SAME: gc "goallc"
; IR: call goabiinternal void @dep.Poll()
; IR: load i64, ptr %p
; IR-NOT: llvm.experimental.gc.statepoint
; IR-NOT: goallc.lto.packages
; CALLBACK: GoALLCStatepoints: ran late pre-isel pipeline for main.Live
; CALLBACK: GoALLCStatepoints: ran late pre-isel pipeline for dep.Poll
; OBJ-LABEL: TEXT main.Live
; OBJ: FUNCDATA_ArgsPointerMaps count=2 bits=1 map[0]=1 map[1]=0
; OBJ: FUNCDATA_LocalsPointerMaps count=2 bits={{[0-9]+}} map[0]=[[ZERO:0+]] map[1]=[[LIVE:0*10*]]
; OBJ-DAG: ordinary safepoint{{.*}}map[1] ArgsPointerMaps=0 LocalsPointerMaps=[[LIVE]]
; OBJ-DAG: stack-growth safepoint{{.*}}map[0] ArgsPointerMaps=1 LocalsPointerMaps=[[ZERO]]
; OBJ-LABEL: TEXT dep.Poll
; OBJ-DAG: ordinary safepoint
; OBJ-LABEL: TEXT dep.AssemblyOnly
declare goabi0 void @"runtime.morestack_noctxt<builtin.246><ABI0>"()
@llvm.compiler.used = appending global [1 x ptr] [ptr @"runtime.morestack_noctxt<builtin.246><ABI0>"], section "llvm.metadata"
declare goabiinternal void @dep.Poll()
define goabiinternal i64 @main.Live(ptr %p) gc "goallc" !goobj.symbol.index !0 {
  call goabiinternal void @dep.Poll()
  %v = load i64, ptr %p
  ret i64 %v
}
!0 = !{i32 7}
!goobj.config = !{!900}
!goobj.imports = !{}
!900 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"codegen-main-id", !"codegen-main", !"1", !"0", !"0", !901, !"0100000000000000"}
!901 = !{}
