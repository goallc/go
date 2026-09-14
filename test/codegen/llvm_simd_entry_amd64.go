// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// The pointer-only signature has no vector ABI floor. Like native Go, this
// implementation relies on its caller to check AVX2 before entering it.
// LLVM-AMD64-LABEL: define goabiinternal void @codegen.simdEntryAdd(
// LLVM-AMD64-SAME: #[[ENTRY:[0-9]+]]
// LLVM-AMD64: add <32 x i8>
// LLVM-AMD64: attributes #[[ENTRY]] = { {{.*}}"target-features"="+avx,+avx2"
// LLVM-OPT-AMD64-NOT: goallc.fmv
// LLVM-OPT-AMD64: define goabiinternal void @codegen.simdEntryAdd(
// LLVM-OPT-AMD64-SAME: #[[ENTRY:[0-9]+]]
// LLVM-OPT-AMD64: attributes #[[ENTRY]] = { {{.*}}"target-features"="+avx,+avx2"
// LLVM-OPT-AMD64-NOT: goallc.fmv
//
//go:noinline
func simdEntryAdd(dst, x, y *[32]int8) {
	archsimd.LoadInt8x32Array(x).Add(archsimd.LoadInt8x32Array(y)).StoreArray(dst)
}
