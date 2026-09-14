target triple = "x86_64-unknown-linux-goobj"

; OPT-NOT: define {{.*}}@lookup(
; OPT-NOT: define {{.*}}@nested(
; OPT-LABEL: define goabiinternal void @caller(
; OPT: call void @llvm.sideeffect(), !goobj.reflect_method
; OPT-LABEL: define goabiinternal void @other_caller(
; OPT: call void @llvm.sideeffect(), !goobj.reflect_method

; REWRITE-NOT: call void @llvm.sideeffect
; REWRITE-LABEL: define goabiinternal void @caller(
; REWRITE-SAME: !goobj.symbol.flags ![[FLAGS:[0-9]+]]
; REWRITE-NOT: call void @llvm.sideeffect
; REWRITE-LABEL: define goabiinternal void @other_caller(
; REWRITE-SAME: !goobj.symbol.flags ![[OTHER:[0-9]+]]
; REWRITE-NOT: call void @llvm.sideeffect
; REWRITE-LABEL: define goabiinternal void @plain(
; REWRITE-NOT: !goobj.symbol.flags
; REWRITE: ret void
; REWRITE-DAG: ![[FLAGS]] = !{i32 33, i32 16}
; REWRITE-DAG: ![[OTHER]] = !{i32 32, i32 0}

declare void @llvm.sideeffect()

define internal goabiinternal void @lookup()
    "go-stack-growth-statepoint" gc "goallc" !goobj.symbol.flags !0 {
  call void @llvm.sideeffect(), !goobj.reflect_method !1
  ret void
}

define internal goabiinternal void @nested()
    "go-stack-growth-statepoint" gc "goallc" {
  call goabiinternal void @lookup()
  ret void
}

define goabiinternal void @caller()
    "go-stack-growth-statepoint" gc "goallc" !goobj.symbol.flags !2 {
  call goabiinternal void @nested()
  call goabiinternal void @lookup()
  ret void
}

define goabiinternal void @other_caller()
    "go-stack-growth-statepoint" gc "goallc" {
  call goabiinternal void @lookup()
  ret void
}

define goabiinternal void @plain()
    "go-stack-growth-statepoint" gc "goallc" {
  ret void
}

; The inlinee's other physical symbol flags must not follow its body.
!0 = !{i32 36, i32 8}
!1 = !{}
!2 = !{i32 1, i32 16}
