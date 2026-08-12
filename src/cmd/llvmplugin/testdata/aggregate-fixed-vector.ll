target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @safepoint()

define goabiinternal ptr @fixed_vector(ptr %source) gc "goallc" {
entry:
  %value = load <2 x ptr>, ptr %source, align 8
  call goabiinternal void @safepoint()
  %result = extractelement <2 x ptr> %value, i32 0
  ret ptr %result
}
