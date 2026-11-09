; CHECK: !goobj.static_rodata_type = !{![[TYPE:[0-9]+]]}
; CHECK: ![[TYPE]] = !{i8 4}
; OBJ: "kind": "SRODATAFIPS"
; OBJ-NOT: "name":
; OBJ: "data_hex": "6500000000000000ca00000000000000"
target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-v128:64:64-v256:64:64-v512:64:64-n8:16:32:64-S64"
target triple = "x86_64-unknown-linux-goobj"
@fips.table = private constant [2 x i64] [i64 101, i64 202]
define goabiinternal i64 @"crypto/internal/fips140/probe.add"(i64 %v) section ".text.fips" !goobj.symbol.index !0 {
  %r = add i64 %v, 1
  ret i64 %r
}
define goabiinternal ptr @"crypto/internal/fips140/probe.tableaddr"() section ".text.fips" !goobj.symbol.index !1 {
  ret ptr @fips.table
}
!0 = !{i32 0, i16 1}
!1 = !{i32 1, i16 1}
!goobj.config = !{!2}
!2 = !{!"fips-id", !"crypto/internal/fips140/probe", !"0", !"0", !"0200000000000000"}
!goobj.imports = !{}
!goobj.static_rodata_type = !{!3}
!3 = !{i8 4}
!llvm.module.flags = !{!4}
!4 = !{i32 1, !"goobj.abi", !5}
!5 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !6}
!6 = !{}
