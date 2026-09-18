; The inline assembly template must never enter gc-live. Actual pointers
; passed to and returned from the assembly must still be relocated.
target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @safepoint()

; CHECK-LABEL: define goabiinternal ptr @leaf_asm(
; CHECK: call goabiinternal token {{.*}}@llvm.experimental.gc.statepoint{{.*}}[ "gc-live"(ptr %p) ]
; CHECK: %[[P:[^ ]+]] = call coldcc ptr @llvm.experimental.gc.relocate
; CHECK: %q = call ptr asm "", "=r,0"(ptr %[[P]])
; CHECK: call goabiinternal token {{.*}}@llvm.experimental.gc.statepoint{{.*}}[ "gc-live"(ptr %q) ]
; CHECK: %[[Q:[^ ]+]] = call coldcc ptr @llvm.experimental.gc.relocate
; CHECK: ret ptr %[[Q]]
define goabiinternal ptr @leaf_asm(ptr %p) gc "goallc" {
entry:
  call goabiinternal void @safepoint()
  %q = call ptr asm "", "=r,0"(ptr %p) "gc-leaf-function"
  call goabiinternal void @safepoint()
  ret ptr %q
}
