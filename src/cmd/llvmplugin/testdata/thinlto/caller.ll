; DUPLICATE: ThinLTO requires unique nonempty module identifiers
; CHECK: @dep.slot = external global i64, {{.*}}!goobj.import ![[REF:[0-9]+]]
; CHECK-LABEL: define {{.*}}@main.result(
; CHECK: ret i64 42
; CHECK-LABEL: define {{.*}}@main.load(
; CHECK: load volatile i64, ptr @dep.slot
; CHECK: tail call goabiinternal void @dep.marked()
; CHECK-NOT: !goobj.marker_reloc
; CHECK-LABEL: define {{.*}}@main.noinlineCaller(
; CHECK: call goabiinternal i64 @dep.add(i64 %v)
; CHECK: declare !goobj.import {{.*}}goabiinternal i64 @dep.add(i64)
; CHECK: !goobj.config = !{![[CONFIG:[0-9]+]]}
; CHECK-DAG: ![[CONFIG]] = !{!"main-id", !"main", !"1", !"0", !"0100000000000000"}
target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
define goabiinternal i64 @main.result() !goobj.symbol.index !0 {
  %r = call goabiinternal i64 @dep.add(i64 41)
  ret i64 %r
}
declare !goobj.import !1 goabiinternal i64 @dep.add(i64)
!0 = !{i32 0, i16 1}
!1 = !{!"dep", i32 0, i32 0}
!goobj.config = !{!2}
!2 = !{!"main-id", !"main", !"1", !"0", !"0100000000000000"}
!goobj.imports = !{!3}
!3 = !{!"dep", !"dep", !"0200000000000000"}
!llvm.module.flags = !{!4}
!4 = !{i32 1, !"goobj.abi", !5}
!5 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !6}
!6 = !{}

define goabiinternal i64 @main.load() !goobj.symbol.index !7 {
  %v = call goabiinternal i64 @dep.load()
  call goabiinternal void @dep.marked()
  ret i64 %v
}
declare !goobj.import !8 goabiinternal i64 @dep.load()
declare !goobj.import !9 goabiinternal void @dep.marked()
!7 = !{i32 1, i16 1}
!8 = !{!"dep", i32 1, i32 0}
!9 = !{!"dep", i32 2, i32 0}
; CHECK-DAG: ![[REF]] = !{!"dep", i32 3, i32 0}

; Keep one imported callee referenced after optimization. The standard ThinLTO
; pipeline must discard its available_externally body without our own rewrite.
define goabiinternal i64 @main.noinlineCaller(i64 %v) noinline optnone !goobj.symbol.index !11 {
  %r = call goabiinternal i64 @dep.add(i64 %v)
  ret i64 %r
}
!11 = !{i32 2, i16 1}
