// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

type lifetimeLocal struct {
	p *int
	x [4]int
}

//go:noescape
func lifetimeObserve(*lifetimeLocal)

//go:noescape
func lifetimeSafepoint()

// The compiler emits these functions in reverse declaration order.
// A loop-carried object's lifetime must not restart at its later uses.
// LLVM-LABEL: define goabiinternal void @codegen.lifetimeCarried(
// LLVM: call void @llvm.lifetime.start.p0(ptr
// LLVM: call void @llvm.memset.inline.p0.i64(
// LLVM: br label
// LLVM-NOT: @llvm.lifetime.start
// LLVM: call goabiinternal void @codegen.lifetimeObserve(
// LLVM-NOT: @llvm.lifetime.start
// LLVM: ret void

// A loop-local object with a complete initial zero can start each iteration.
// LLVM-LABEL: define goabiinternal void @codegen.lifetimeLoop(
// LLVM: alloca %codegen.lifetimeLocal
// LLVM-NOT: @llvm.lifetime.start
// LLVM: br label
// LLVM: phi i64
// LLVM: call void @llvm.lifetime.start.p0(ptr
// LLVM: call void @llvm.memset.inline.p0.i64(
// LLVM: call goabiinternal void @codegen.lifetimeObserve(
// LLVM-NOT: @llvm.lifetime.start
// LLVM: ret void

// Allocation is fixed, but the object is not initialized or live on the
// early call/skip path. Later whole-variable assignments are not new starts.
// LLVM-LABEL: define goabiinternal void @codegen.lifetimeBranch(
// LLVM: alloca %codegen.lifetimeLocal
// LLVM-NOT: @llvm.lifetime.start
// LLVM: call goabiinternal void @codegen.lifetimeSafepoint()
// LLVM: br i1
// LLVM: ret void
// LLVM: call void @llvm.lifetime.start.p0(ptr
// LLVM: call void @llvm.memset.inline.p0.i64(
// LLVM: call goabiinternal void @codegen.lifetimeObserve(
// LLVM-NOT: @llvm.lifetime.start
// LLVM: call goabiinternal void @codegen.lifetimeObserve(
// LLVM-NOT: @llvm.fake.use
// LLVM: br label
//
// LLVM-OPT-LABEL: define goabiinternal void @codegen.lifetimeBranch(
// LLVM-OPT: alloca %codegen.lifetimeLocal
// LLVM-OPT-NOT: @llvm.lifetime.start
// LLVM-OPT-NOT: @llvm.memset
// LLVM-OPT: call goabiinternal void @codegen.lifetimeSafepoint()
// LLVM-OPT: br i1
// LLVM-OPT: call void @llvm.lifetime.start.p0(ptr
// LLVM-OPT: call void @llvm.memset.inline.p0.i64(
// LLVM-OPT: call goabiinternal void @codegen.lifetimeObserve(
// LLVM-OPT-NOT: @llvm.lifetime.start
// LLVM-OPT: call goabiinternal void @codegen.lifetimeObserve(
// LLVM-OPT-NOT: @llvm.fake.use
// LLVM-OPT: br label
//
//go:noinline
func lifetimeBranch(take bool) {
	lifetimeSafepoint()
	if take {
		var local lifetimeLocal
		lifetimeObserve(&local)
		local = lifetimeLocal{}
		lifetimeObserve(&local)
	}
}

//go:noinline
func lifetimeLoop(n int) {
	for i := 0; i < n; i++ {
		var local lifetimeLocal
		local.x[0] = i
		lifetimeObserve(&local)
	}
}

//go:noinline
func lifetimeCarried(n int) {
	var local lifetimeLocal
	for i := 0; i < n; i++ {
		lifetimeObserve(&local)
		local.x[0]++
	}
}
