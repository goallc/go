// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-AMD64-DAG: call <2 x i64> @llvm.x86.pclmulqdq({{.*}}i8 17)
// LLVM-AMD64-DAG: !{!"x86.avxpclmulqdq"}
// LLVM-AMD64-DAG: !{!"x86.avx"}
// LLVM-AMD64-DAG: !{!"x86.pclmulqdq"}
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.clmul<goallc.fmv.avx-pclmulqdq>"
// LLVM-OPT-AMD64-DAG: "target-features"="{{[^"]*}}+avx{{[^"]*}}+pclmul{{[^"]*}}"
// LLVM-ASM-AMD64-DAG: VPCLMULQDQ
//
//go:noinline
func clmul(x, y archsimd.Uint64x2) archsimd.Uint64x2 {
	if !archsimd.X86.AVXPCLMULQDQ() {
		return x
	}
	return x.CarrylessMultiplyOdd(y)
}

// A local zero vector may be hoisted into entry. It is not a function-wide
// AVX precondition: the scalar/array ABI still supports non-AVX callers.
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.guardedLocalZero<goallc.fmv.baseline>"({{.*}}) [[BASEATTR:#[0-9]+]]
// LLVM-OPT-AMD64-DAG: attributes [[BASEATTR]] = { {{[^}]*}}"target-cpu"="x86-64" }
//
//go:noinline
func guardedLocalZero(x, y [4]int32, choose bool) (out [4]int32) {
	if !archsimd.X86.AVX() {
		return
	}
	a, b := archsimd.LoadInt32x4(x[:]), archsimd.LoadInt32x4(y[:])
	var result archsimd.Int32x4
	if choose {
		result = a.Add(b)
	}
	result.Store(out[:])
	return
}
