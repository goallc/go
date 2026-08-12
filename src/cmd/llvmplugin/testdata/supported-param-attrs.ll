target triple = "aarch64-unknown-linux-goobj"

%stack_arg = type [5 x ptr]

declare goabiinternal void @supported_callee(ptr)
declare goabiinternal void @supported_byval_callee(
    ptr byval(%stack_arg) align 8)
declare goabiinternal void @supported_byref_callee(
    ptr byref(%stack_arg) align 8) #1

define goabiinternal void @supported_param_attrs(ptr %argument) #0 gc "goallc" {
entry:
  call goabiinternal void @supported_callee(ptr noundef nonnull readnone align 8 %argument)
  ret void
}

define goabiinternal void @supported_byval_attr(ptr %argument) #0 gc "goallc" {
entry:
  %stack_arg = alloca %stack_arg, align 8
  store %stack_arg zeroinitializer, ptr %stack_arg, align 8
  %first = getelementptr inbounds %stack_arg, ptr %stack_arg, i32 0, i32 0
  store ptr %argument, ptr %first, align 8
  call goabiinternal void @supported_byval_callee(
      ptr byval(%stack_arg) align 8 %stack_arg)
  ret void
}

define goabiinternal void @supported_byref_attr() #0 gc "goallc" {
entry:
  %stack_result = alloca %stack_arg, align 8
  call goabiinternal void @supported_byref_callee(
      ptr byref(%stack_arg) align 8 %stack_result) #1
  ret void
}

attributes #0 = { "frame-pointer"="non-leaf" }
attributes #1 = { "go_memory_results"="0" }
