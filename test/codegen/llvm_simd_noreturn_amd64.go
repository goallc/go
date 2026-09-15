// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import (
	"simd/archsimd"
	"testing"
)

// Skip's LLVM attribute, not a Go SSA rewrite, terminates the unsupported path.
// LLVM-AMD64: declare {{.*}}@"testing.(*common).Skip"({{.*}}) #[[NR:[0-9]+]]
// LLVM-AMD64: attributes #[[NR]] = { {{.*}}noreturn
// LLVM-OPT-AMD64: define internal goabiinternal void @"codegen.simdAfterSkip<goallc.fmv.baseline>"(
// LLVM-OPT-AMD64: call {{.*}}@"testing.(*common).Skip"(
// LLVM-OPT-AMD64-NEXT: unreachable
// LLVM-OPT-AMD64: define internal goabiinternal void @"codegen.simdAfterSkip<goallc.fmv.avx2>"(
// LLVM-OPT-AMD64: add <32 x i8>
// LLVM-OPT-AMD64: ret void
//
//go:noinline
func simdAfterSkip(t *testing.T, dst, x, y *[32]int8) {
	if !archsimd.X86.AVX2() {
		t.Skip()
	}
	archsimd.LoadInt8x32Array(x).Add(archsimd.LoadInt8x32Array(y)).StoreArray(dst)
}

// The dead SIMD operation must not leave a resolver or feature versions.
// LLVM-NM-AMD64-NOT: codegen.simdDeadAfterSkip.goallc.fmv.slot
//
//go:noinline
func simdDeadAfterSkip(t *testing.T, dst, x, y *[32]int8) {
	t.Skip()
	archsimd.LoadInt8x32Array(x).Add(archsimd.LoadInt8x32Array(y)).StoreArray(dst)
}
