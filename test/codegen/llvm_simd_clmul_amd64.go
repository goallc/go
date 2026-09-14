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
// LLVM-ASM-AMD64-DAG: VPCLMULQDQ {{.*X[0-9]}}
//
//go:noinline
func clmul(x, y archsimd.Uint64x2) archsimd.Uint64x2 {
	if !archsimd.X86.AVXPCLMULQDQ() {
		return x
	}
	return x.CarrylessMultiplyOdd(y)
}

// LLVM-AMD64-DAG: call <4 x i64> @llvm.x86.pclmulqdq.256({{.*}}i8 1)
// LLVM-AMD64-DAG: !{!"x86.vpclmulqdq"}
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.clmul256<goallc.fmv.vpclmulqdq>"({{.*}}) [[ATTR256:#[0-9]+]]
// LLVM-OPT-AMD64-DAG: attributes [[ATTR256]] = { {{.*}}"target-features"="{{[^"]*}}+vpclmulqdq{{[^"]*}}"
// LLVM-ASM-AMD64-DAG: VPCLMULQDQ {{.*Y[0-9]}}
//
//go:noinline
func clmul256(x, y archsimd.Uint64x4) archsimd.Uint64x4 {
	if !archsimd.X86.VPCLMULQDQ() {
		return x
	}
	return x.CarrylessMultiplyOddEven(y)
}

// LLVM-AMD64-DAG: call <8 x i64> @llvm.x86.pclmulqdq.512({{.*}}i8 16)
// LLVM-AMD64-DAG: !{!"x86.avx512vpclmulqdq"}
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.clmul512<goallc.fmv.avx512vpclmulqdq>"({{.*}}) [[ATTR512:#[0-9]+]]
// LLVM-OPT-AMD64-DAG: attributes [[ATTR512]] = { {{.*}}"target-features"="{{[^"]*}}+avx512f{{[^"]*}}+vpclmulqdq{{[^"]*}}"
// LLVM-ASM-AMD64-DAG: VPCLMULQDQ {{.*Z[0-9]}}
//
//go:noinline
func clmul512(x, y archsimd.Uint64x8) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512VPCLMULQDQ() {
		return x
	}
	return x.CarrylessMultiplyEvenOdd(y)
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
