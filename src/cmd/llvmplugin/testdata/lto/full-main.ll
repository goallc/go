; COMMON: @shared = dso_local global [4 x i64] zeroinitializer, section ".noptrbss", align 32, !goobj.symbol.nonpackage
; COMMON-SAME: !goobj.gotype ![[TYPE:[0-9]+]]
; COMMON: ![[TYPE]] = !{ptr @type.large}
; INIT: @shared = dso_local global [4 x i64] [i64 1, i64 2, i64 3, i64 4], section ".data"
; INIT-SAME: !goobj.gotype ![[TYPE:[0-9]+]]
; INIT: ![[TYPE]] = !{ptr @type.initialized}
; LINKNAME-LABEL: define dso_local goabiinternal {{.*}}i64 @target()
; LINKNAME: ret i64 42
; FullLTO checks observe the single output module and retained package manifest.
; CROSS-LABEL: define dso_local goabiinternal {{.*}}i64 @caller.Answer()
; CROSS-NOT: call
; CROSS: ret i64 42
; CROSS-LABEL: define dso_local goabiinternal {{.*}}i64 @caller.Opaque
; CROSS: call goabiinternal i64 @callee.Opaque
; CROSS-LABEL: define dso_local goabiinternal {{.*}}i64 @caller.Restricted
; CROSS: call goabiinternal i64 @callee.Restricted
; CROSS: define dso_local goabiinternal {{.*}}i64 @callee.Opaque
; CROSS: define dso_local goabiinternal {{.*}}i64 @callee.Restricted
; CROSS: call goabiinternal i64 @"runtime.private<linkname>"
; CROSS-NOT: goallc.lto.owner
;
; META-DAG: @leaf.counter = {{.*}}global i64 41, {{.*}}!goobj.{{(keep|gotype)}} ![[KEEP:[0-9]+]], !goobj.{{(keep|gotype)}} ![[KEEP]]
; META-DAG: @leaf.identity = {{.*}}constant i8 7
; META-DAG: @leaf.unreferenced = {{.*}}global i64 3
; META-LABEL: define dso_local goabiinternal {{.*}}ptr @caller.Identity()
; META-NOT: call
; META: ret ptr @leaf.identity
; META-LABEL: define dso_local goabiinternal i64 @caller.Read()
; META-NOT: call
; META: load volatile i64, ptr @leaf.counter
; META-DAG: !goobj.imports = !{}
; META: ![[KEEP]] = !{ptr @leaf.identity}
; META-NOT: goallc.lto.owner
;
; WEAK: @choice = weak dso_local global i64 3
; OTHER: @choice = weak dso_local global i64 5
; BAD-PIPELINE: goallc-lto: unknown pass name 'not-a-real-pass'
; RESOLVE: @choice = dso_local global i64 6
; RESOLVE-SAME: !goobj.gotype ![[TYPE:[0-9]+]], !goobj.keep ![[TYPE]], !goobj.marker_relocs ![[MARK:[0-9]+]]
; RESOLVE-NOT: @choice.{{[0-9]+}}
; RESOLVE-DAG: ![[TYPE]] = !{ptr @type.strong}
; RESOLVE-DAG: ![[MARK]] = !{![[EDGE:[0-9]+]]}
; RESOLVE-DAG: ![[EDGE]] = !{ptr @type.strong, i32 23, i64 0}
; BSS: @shared = dso_local global [2 x i64] [i64 4, i64 5], section ".data", !goobj.symbol.nonpackage
;
; ABI: %go.abi.pad{{(\.[0-9]+)?}} = type <{ i8 }>
; ABI: %callee.Byte = type { i8 }
; ABI-LABEL: define dso_local goabiinternal {{.*}}i64 @caller.Padded(%go.abi.pad
; ABI-LABEL: define dso_local goabiinternal {{.*}}ptr @caller.Closure()
; ABI: ret ptr @callee.Closure.goallc.lto.callee
; ABI-LABEL: define dso_local goabiinternal {{.*}}i8 @callee.Byte(%callee.Byte
; ABI: define dso_local goabiinternal {{.*}}i64 @callee.Closure.goallc.lto.callee(i64
; ABI-NOT: !goobj.content_addressable
; ABI0-LABEL: define dso_local goabiinternal {{.*}}ptr @caller.ABI0()
; ABI0: ret ptr @"callee.ABI0.goallc.lto.callee<ABI0>"
; ABI0-LABEL: define dso_local goabi0 void @"callee.ABI0.goallc.lto.callee<ABI0>"()
; ABI0-SAME: !goobj.symbol.index ![[ABI0INDEX:[0-9]+]]
; ABI0: ![[ABI0INDEX]] = !{i32 {{[0-9]+}}, i16 0}
;
; MANIFEST-DAG: "imports":[{"fingerprint":"0200000000000000","path":"callee","prefix":"callee"}]
; MANIFEST-DAG: "imports":[{"fingerprint":"0300000000000000","path":"leaf","prefix":"leaf"}]
target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
!goobj.config = !{!0}
!goobj.imports = !{}
!0 = !{!"main-id", !"main", !"1", !"0", !"0400000000000000"}
!1 = !{}

!llvm.module.flags = !{!2}
!2 = !{i32 1, !"goobj.abi", !3}
!3 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !1}
