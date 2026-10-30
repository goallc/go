; ERROR: goallc-lto: FIPS packages must use ordinary object compilation: crypto/internal/fips140/probe
target triple = "x86_64-unknown-linux-goobj"
!goobj.config = !{!0}
!goobj.static_rodata_type = !{!2}
!0 = !{!"id", !"crypto/internal/fips140/probe", !"0", !"0", !"0100000000000000"}
!1 = !{}
!2 = !{i8 4}

!llvm.module.flags = !{!3}
!3 = !{i32 1, !"goobj.abi", !4}
!4 = !{!"goallc.goobj", !"linux", !"amd64", !"test", !"GOAMD64", !"v1", !"0", !1}
