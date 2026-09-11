// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && amd64

package codegen

import "simd/archsimd"

// Both enabled edges must be specialized before LLVM simplifies the OR join.
// The scalar signature establishes no unconditional vector feature floor.
// LLVM-AMD64-LABEL: define goabiinternal void @codegen.llvmGuardPaths(
// LLVM-AMD64-SAME: #[[PATHS:[0-9]+]]
// LLVM-AMD64: add <32 x i8> {{.*}}!goallc.cpu.requires ![[LOW:[0-9]+]]
// LLVM-AMD64: attributes #[[PATHS]] = { {{.*}}"goallc.cpu.multiversion"="x86.avx2,x86.avx512"
// LLVM-AMD64: ![[LOW]] = !{!"x86.avx2"}
// LLVM-NM-AMD64: codegen.llvmGuardPaths.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-5: codegen.llvmGuardPaths<1>
// LLVM-NM-AMD64-NOT: codegen.llvmGuardPaths<1>

//go:noinline
func llvmGuardPaths(x, y, out *[32]int8) {
	if archsimd.X86.AVX512() || archsimd.X86.AVX2() {
		a := archsimd.LoadInt8x32Array(x)
		b := archsimd.LoadInt8x32Array(y)
		a.Add(b).StoreArray(out)
		return
	}
	*out = *x
}
