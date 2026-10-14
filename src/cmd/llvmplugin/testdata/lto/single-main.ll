; One optimized module, with distinct output indices for formerly overlapping
; package symbol spaces. The manifest retains both original package indices.
; CHECK-LABEL: define goabiinternal{{.*}} i64 @main.Answer()
; CHECK: ret i64 42
; CHECK-LABEL: define goabiinternal i64 @main.Opaque()
; CHECK: call goabiinternal i64 @dep.Opaque()
; CHECK: define goabiinternal i64 @dep.Opaque()
; CHECK: !goobj.config = !{
; CHECK-NOT: module asm
; MANIFEST-DAG: "name":"main.Answer","original_index":7,"package":0
; MANIFEST-DAG: "name":"dep.Add","original_index":7,"package":1
; MANIFEST-DAG: "prefix":"main"
; MANIFEST-DAG: "prefix":"dep"
; MANIFEST-DAG: "version":1
target triple = "x86_64-unknown-linux-goobj"
define goabiinternal i64 @main.Answer() !goobj.symbol.index !1 {
  %v = call goabiinternal i64 @dep.Add(i64 35)
  ret i64 %v
}
define goabiinternal i64 @main.Opaque() !goobj.symbol.index !2 {
  %v = call goabiinternal i64 @dep.Opaque()
  ret i64 %v
}
declare !goobj.import !3 goabiinternal i64 @dep.Add(i64)
declare !goobj.import !4 goabiinternal i64 @dep.Opaque()
!goobj.config = !{!0}
!goobj.imports = !{!5}
!0 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"main-id", !"main", !"1", !"0", !"0", !6, !"0100000000000000"}
!1 = !{i32 7}
!2 = !{i32 8}
!3 = !{!"dep", i32 7, i8 0}
!4 = !{!"dep", i32 8, i8 0}
!5 = !{!"dep", !"dep", !"0200000000000000"}
!6 = !{}
