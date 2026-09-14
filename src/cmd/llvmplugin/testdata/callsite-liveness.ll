target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal ptr @produce(ptr)
declare goabiinternal void @consume(ptr)
declare goabiinternal void @safepoint()
declare goabiinternal void @leaf(ptr) #0

; The first call needs only p; its own argument and result are excluded.
; The second needs only the first call result, not its argument p. The leaf
; call between them must still contribute its operands to backward liveness.
; IR-LABEL: define goabiinternal ptr @changing_live_sets(
; IR: @llvm.experimental.gc.statepoint{{.*}}@produce,{{.*}}[ "gc-live"(ptr %p) ]
; IR: %[[RESULT:[0-9]+]] = call ptr @llvm.experimental.gc.result
; IR: call goabiinternal void @leaf(ptr %p.relocated)
; IR: @llvm.experimental.gc.statepoint{{.*}}@consume,{{.*}}[ "gc-live"(ptr %[[RESULT]]) ]
; IR: @llvm.experimental.gc.statepoint{{.*}}@safepoint,{{.*}}[ "gc-live"(ptr %result.relocated{{[0-9]+}}) ]
; IR: ret ptr %result.relocated

define goabiinternal ptr @changing_live_sets(ptr %argument, ptr %p) gc "goallc" {
entry:
  %result = call goabiinternal ptr @produce(ptr %argument)
  call goabiinternal void @leaf(ptr %p)
  call goabiinternal void @consume(ptr %p)
  call goabiinternal void @safepoint()
  ret ptr %result
}

; Both calls in each predecessor must retain its successor PHI edge operand.
; Changing blocks must discard the previous block's snapshots.
; IR-LABEL: define goabiinternal ptr @phi_edge_live_sets(
; IR: @llvm.experimental.gc.statepoint{{.*}}[ "gc-live"(ptr %p) ]
; IR: @llvm.experimental.gc.statepoint{{.*}}[ "gc-live"(ptr %p.relocated{{[0-9]+}}) ]
; IR: @llvm.experimental.gc.statepoint{{.*}}[ "gc-live"(ptr %q) ]
; IR: @llvm.experimental.gc.statepoint{{.*}}[ "gc-live"(ptr %q.relocated{{[0-9]+}}) ]
; IR: %result = phi ptr
; IR: @llvm.experimental.gc.statepoint{{.*}}[ "gc-live"(ptr %result) ]
; IR: ret ptr %result.relocated

define goabiinternal ptr @phi_edge_live_sets(ptr %p, ptr %q, i1 %choose) gc "goallc" {
entry:
  br i1 %choose, label %left, label %right
left:
  call goabiinternal void @safepoint()
  call goabiinternal void @safepoint()
  br label %join
right:
  call goabiinternal void @safepoint()
  call goabiinternal void @safepoint()
  br label %join
join:
  %result = phi ptr [ %p, %left ], [ %q, %right ]
  call goabiinternal void @safepoint()
  ret ptr %result
}

attributes #0 = { "gc-leaf-function" }
