target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @unsupported_callee(ptr noalias)

define goabiinternal void @unsupported_param_attr(ptr %context) gc "goallc" {
entry:
  call goabiinternal void @unsupported_callee(ptr noalias %context)
  ret void
}
