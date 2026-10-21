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
; META-DAG: @leaf.counter = {{.*}}global i64 41
; META-DAG: @leaf.identity = {{.*}}constant i8 7
; META-DAG: @leaf.unreferenced = {{.*}}global i64 3
; META-LABEL: define dso_local goabiinternal {{.*}}ptr @caller.Identity()
; META-NOT: call
; META: ret ptr @leaf.identity
; META-LABEL: define dso_local goabiinternal i64 @caller.Read()
; META-NOT: call
; META: load volatile i64, ptr @leaf.counter
; META-DAG: !goobj.imports = !{}
; META-DAG: !goobj.keep = !{![[KEEP:[0-9]+]]}
; META-DAG: !goobj.gotype = !{![[KEEP]]}
; META: ![[KEEP]] = !{ptr @leaf.counter, ptr @leaf.identity}
; META-NOT: goallc.lto.owner
;
; WEAK: @choice = weak dso_local global i64 3
; OTHER: @choice = weak dso_local global i64 5
; BAD-PIPELINE: goallc-lto: unknown pass name 'not-a-real-pass'
; RESOLVE: @choice = dso_local global i64 6
; RESOLVE-NOT: @choice.{{[0-9]+}}
; BSS: @shared = dso_local global [2 x i64] [i64 4, i64 5], section ".data", !goobj.symbol.nonpackage
;
; ABI: %go.abi.pad = type <{ i8 }>
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
!0 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"main-id", !"main", !"1", !"0", !"0", !1, !"0400000000000000"}
!1 = !{}
