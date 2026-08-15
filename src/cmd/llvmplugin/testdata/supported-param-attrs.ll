target triple = "aarch64-unknown-linux-goobj"

%stack_arg = type [5 x ptr]

declare goabiinternal void @supported_callee(ptr)
declare token @llvm.call.preallocated.setup(i32)
declare ptr @llvm.call.preallocated.arg(token, i32)
declare goabiinternal void @supported_preallocated_callee(
    ptr preallocated(%stack_arg) align 8)
declare goabiinternal void @supported_goret_callee(
    ptr goret(%stack_arg) "goretindex"="0" align 8)

define goabiinternal void @supported_param_attrs(ptr %argument) #0 gc "goallc" {
entry:
  call goabiinternal void @supported_callee(ptr noundef nonnull readnone align 8 %argument)
  ret void
}

define goabiinternal void @supported_preallocated_attr(ptr %argument) #0 gc "goallc" {
entry:
  %setup = call token @llvm.call.preallocated.setup(i32 1)
  %home = call ptr @llvm.call.preallocated.arg(token %setup, i32 0) preallocated(%stack_arg)
  store %stack_arg zeroinitializer, ptr %home, align 8
  %first = getelementptr inbounds %stack_arg, ptr %home, i32 0, i32 0
  store ptr %argument, ptr %first, align 8
  call goabiinternal void @supported_preallocated_callee(
      ptr preallocated(%stack_arg) align 8 %home)
      ["preallocated"(token %setup)]
  ret void
}

define goabiinternal void @supported_goret_attr() #0 gc "goallc" {
entry:
  %stack_result = alloca %stack_arg, align 8
  call goabiinternal void @supported_goret_callee(
      ptr goret(%stack_arg) "goretindex"="0" align 8 %stack_result)
  ret void
}

attributes #0 = { "frame-pointer"="non-leaf" }
