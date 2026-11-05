; IR-LABEL: define goabiinternal i1 @mixed_phi(
; IR: %base = phi ptr [ %local, %default ], [ %config, %entry ]
; IR-NOT: .base = phi
; IR-NOT: .offset = phi
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR-SAME: "gc-live"(ptr %base)
; IR: %base.relocated{{.*}} = call {{.*}}@llvm.experimental.gc.relocate
; IR: getelementptr i8, ptr %base.relocated{{.*}}, i64 8
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR: getelementptr i8, ptr %base.relocated{{.*}}, i64 24
; IR-LABEL: define goabiinternal i1 @mixed_select(
; IR: %base = select i1 %choose, ptr %local, ptr %config
; IR-NOT: .base = select
; IR-NOT: .offset = select
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR-SAME: "gc-live"(ptr %base)
; IR: %base.relocated{{.*}} = call {{.*}}@llvm.experimental.gc.relocate
; IR: getelementptr i8, ptr %base.relocated{{.*}}, i64 8
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint
; IR: getelementptr i8, ptr %base.relocated{{.*}}, i64 24

; The early Go plugin opts into shared frame bases before ordinary SROA.
; OPT-LABEL: define goabiinternal i1 @mixed_phi(
; OPT: %local = alloca { i64, ptr, i64, ptr }
; OPT-NOT: alloca
; OPT: %base = phi ptr [ %local, %default ], [ %config, %entry ]
; OPT-NOT: phi
; OPT: call goabiinternal void @safepoint()
; OPT: %a = getelementptr i8, ptr %base, i64 8
; OPT: call goabiinternal void @safepoint()
; OPT: %b = getelementptr i8, ptr %base, i64 24
; OPT-LABEL: define goabiinternal i1 @mixed_select(
; OPT: %local = alloca { i64, ptr, i64, ptr }
; OPT-NOT: alloca
; OPT: %base = select i1 %choose, ptr %local, ptr %config
; OPT-NOT: select
; OPT: call goabiinternal void @safepoint()
; OPT: %a = getelementptr i8, ptr %base, i64 8
; OPT: call goabiinternal void @safepoint()
; OPT: %b = getelementptr i8, ptr %base, i64 24

target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @safepoint()


define goabiinternal i1 @mixed_phi(ptr %config) gc "goallc" {
entry:
  %local = alloca {i64, ptr, i64, ptr}, align 8
  %nil = icmp eq ptr %config, null
  br i1 %nil, label %default, label %merge
default:
  store {i64, ptr, i64, ptr} zeroinitializer, ptr %local
  br label %merge
merge:
  %base = phi ptr [%local, %default], [%config, %entry]
  call goabiinternal void @safepoint()
  %a = getelementptr i8, ptr %base, i64 8
  %x = load ptr, ptr %a
  call goabiinternal void @safepoint()
  %b = getelementptr i8, ptr %base, i64 24
  %y = load ptr, ptr %b
  %eq = icmp eq ptr %x, %y
  ret i1 %eq
}


define goabiinternal i1 @mixed_select(ptr %config, i1 %choose) gc "goallc" {
entry:
  %local = alloca {i64, ptr, i64, ptr}, align 8
  store {i64, ptr, i64, ptr} zeroinitializer, ptr %local
  %base = select i1 %choose, ptr %local, ptr %config
  call goabiinternal void @safepoint()
  %a = getelementptr i8, ptr %base, i64 8
  %x = load ptr, ptr %a
  call goabiinternal void @safepoint()
  %b = getelementptr i8, ptr %base, i64 24
  %y = load ptr, ptr %b
  %eq = icmp eq ptr %x, %y
  ret i1 %eq
}
