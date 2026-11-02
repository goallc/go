; CHECK-LABEL: define {{.*}}@main.result(
; CHECK: ret i64 42
; OBJ: "kind": "SRODATA"
; OBJ-NOT: "name":
; OBJ: "data_hex": "2a000000000000000d00000000000000"
; This ordinary package imports a FIPS function, but its generated static data
; must not inherit the other package's FIPS classification.
target triple = "x86_64-unknown-linux-goobj"
@main.table = private constant [2 x i64] [i64 42, i64 13]
define goabiinternal i64 @main.result() !goobj.symbol.index !0 {
  %r = call goabiinternal i64 @"crypto/internal/fips140/probe.add"(i64 41)
  ret i64 %r
}
define goabiinternal ptr @main.tableaddr() !goobj.symbol.index !1 {
  ret ptr @main.table
}
declare !goobj.import !2 goabiinternal i64 @"crypto/internal/fips140/probe.add"(i64)
!0 = !{i32 0, i16 1}
!1 = !{i32 1, i16 1}
!2 = !{!"crypto/internal/fips140/probe", i32 0, i32 0}
!goobj.config = !{!3}
!3 = !{!"main-id", !"main", !"1", !"0", !"0100000000000000"}
!goobj.imports = !{!4}
!4 = !{!"crypto/internal/fips140/probe", !"crypto/internal/fips140/probe", !"0200000000000000"}
!llvm.module.flags = !{!5}
!5 = !{i32 1, !"goobj.abi", !6}
!6 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !7}
!7 = !{}
