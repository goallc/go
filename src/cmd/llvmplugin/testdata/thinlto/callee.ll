; CHECK-LABEL: define {{.*}}@dep.add(
; CHECK-SAME: !goobj.symbol.index ![[INDEX:[0-9]+]]
; CHECK: !goobj.config = !{![[CONFIG:[0-9]+]]}
; CHECK-DAG: ![[INDEX]] = !{i32 0, i16 1}
; CHECK-DAG: ![[CONFIG]] = !{!"dep-id", !"dep", !"0", !"0", !"0200000000000000"}
target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-v128:64:64-v256:64:64-v512:64:64-n8:16:32:64-S64"
define goabiinternal i64 @dep.add(i64 %v) !goobj.symbol.index !0 !goobj.symbol.flags !11 {
  %r = add i64 %v, 1
  ret i64 %r
}
@dep.slot = global i64 41, !goobj.symbol.index !10, !goobj.symbol.flags !11

define goabiinternal i64 @dep.load() !goobj.symbol.index !7 {
  %v = load volatile i64, ptr @dep.slot
  ret i64 %v
}

; Go reachability markers must stay in their owning module: LLVM's summary
; does not describe the reference in the instruction metadata.
@marker.target = external global i8
declare void @llvm.sideeffect()
define goabiinternal void @dep.marked() !goobj.symbol.index !8 {
  call void @llvm.sideeffect(), !goobj.marker_reloc !9
  ret void
}
!10 = !{i32 3, i16 0}
!7 = !{i32 1, i16 1}
!8 = !{i32 2, i16 1}
!9 = !{ptr @marker.target, i32 24, i64 0}
!0 = !{i32 0, i16 1}
!goobj.config = !{!2}
!2 = !{!"dep-id", !"dep", !"0", !"0", !"0200000000000000"}
!goobj.imports = !{}
!llvm.module.flags = !{!4}
!4 = !{i32 1, !"goobj.abi", !5}
!5 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !6}
!6 = !{}

; Preserve UsedInIface in both function and global cross-package references.
!11 = !{i8 0, i8 1}

; Non-leaf codegen needs the compiler-owned late stack-growth declaration.
declare goabi0 void @"runtime.morestack_noctxt<builtin.246><ABI0>"()
@llvm.compiler.used = appending global [2 x ptr] [ptr @"runtime.morestack_noctxt<builtin.246><ABI0>", ptr @marker.target], section "llvm.metadata"
