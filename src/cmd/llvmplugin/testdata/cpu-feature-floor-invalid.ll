; CHECK: error: GoALLC CPU feature floor x86.avx does not match module architecture arm64

target triple = "aarch64-unknown-linux-gnu"

define void @bad() #0 {
entry:
  ret void
}

attributes #0 = { "goallc.cpu.feature-floor"="x86.avx" }

!goallc.cpu.config = !{!0}
!0 = !{!"goallc.cpu.v1", !"arm64", !"v8.0"}
