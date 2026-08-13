target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @callee()
declare i32 @__gxx_personality_v0(...)

define goabiinternal void @unsupported_invoke()
    gc "goallc"
    personality ptr @__gxx_personality_v0 {
entry:
  invoke goabiinternal void @callee()
      to label %done unwind label %unwind

done:
  ret void

unwind:
  %landingpad = landingpad { ptr, i32 }
      cleanup
  resume { ptr, i32 } %landingpad
}
