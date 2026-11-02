; Three copies with the same final name: two equal bodies and one different.
; The input names already distinguish them; LTO must preserve those names.
;
; IR-DAG: @main.closure = {{.*}}ptr @"dep.Closure#[[A:AQAAAAAAAAA=]]#"
; IR-DAG: @main.different = {{.*}}ptr @"dep.Closure#[[B:AgAAAAAAAAA=]]#"
; IR-DAG: @dep.closure = {{.*}}ptr @"dep.Closure#[[C:AwAAAAAAAAA=]]#"
; IR: define {{.*}} @"dep.Closure#[[A]]#"()
; IR-SAME: !goobj.content_addressable
; IR: ret i64 11
; IR: define {{.*}} @"dep.Closure#[[B]]#"()
; IR-SAME: !goobj.content_addressable
; IR: ret i64 22
; IR: define {{.*}} @"dep.Closure#[[C]]#"()
; IR-SAME: !goobj.content_addressable
; IR: ret i64 11
; MANIFEST: "name":"dep.Closure","package":1
; MANIFEST-NOT: original_name
;
; OBJ: class=hashed class_index=0 name="dep.Closure" abi=1 kind=STEXT{{.*}} hash=[[SAME:[a-f0-9]+]]
; OBJ-NOT: class_index=1 {{.*}}hash=[[SAME]]
; OBJ: class=hashed class_index=1 name="dep.Closure" abi=1 kind=STEXT{{.*}} hash={{[a-f0-9]+}}
; OBJ: class=hashed class_index=2 name="dep.Closure" abi=1 kind=STEXT{{.*}} hash=[[SAME]]
; OBJ: "main.closure" reloc[0] {{.*}}pkg_kind=hashed sym_index=0 name="dep.Closure"
; OBJ: "main.different" reloc[0] {{.*}}pkg_kind=hashed sym_index=1 name="dep.Closure"
; OBJ: "dep.closure" reloc[0] {{.*}}pkg_kind=hashed sym_index=2 name="dep.Closure"
target triple = "x86_64-unknown-linux-goobj"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n8:16:32:64-S128"
!goobj.config = !{!0}
!goobj.imports = !{}
!0 = !{!"id", !"main", !"1", !"0", !"0100000000000000"}
!1 = !{i1 true}
!2 = !{i32 0}
!3 = !{}
!llvm.module.flags = !{!4}
!4 = !{i32 1, !"goobj.abi", !5}
!5 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !3}
@main.closure = global ptr @"dep.Closure#AQAAAAAAAAA=#", !goobj.symbol.index !2
define goabiinternal i64 @"dep.Closure#AQAAAAAAAAA=#"() noinline !goobj.content_addressable !1 {
  ret i64 11
}
@main.different = global ptr @"dep.Closure#AgAAAAAAAAA=#", !goobj.symbol.index !6
define goabiinternal i64 @"dep.Closure#AgAAAAAAAAA=#"() noinline !goobj.content_addressable !1 {
  ret i64 22
}
!6 = !{i32 1}
