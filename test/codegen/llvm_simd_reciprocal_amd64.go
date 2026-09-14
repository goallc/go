// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-AMD64-DAG: call <4 x float> @llvm.x86.sse.rcp.ps(
// LLVM-ASM-AMD64-DAG: VRCPPS {{.*X[0-9]}}

// LLVM-AMD64-DAG: call <4 x float> @llvm.x86.sse.rsqrt.ps(
// LLVM-ASM-AMD64-DAG: VRSQRTPS {{.*X[0-9]}}
//
//go:noinline
func recip32x4(x archsimd.Float32x4) (archsimd.Float32x4, archsimd.Float32x4) {
	if !archsimd.X86.AVX() {
		return x, x
	}
	return x.Reciprocal(), x.ReciprocalSqrt()
}

// LLVM-AMD64-DAG: call <8 x float> @llvm.x86.avx.rcp.ps.256(
// LLVM-ASM-AMD64-DAG: VRCPPS {{.*Y[0-9]}}

// LLVM-AMD64-DAG: call <8 x float> @llvm.x86.avx.rsqrt.ps.256(
// LLVM-ASM-AMD64-DAG: VRSQRTPS {{.*Y[0-9]}}
//
//go:noinline
func recip32x8(x archsimd.Float32x8) (archsimd.Float32x8, archsimd.Float32x8) {
	if !archsimd.X86.AVX() {
		return x, x
	}
	return x.Reciprocal(), x.ReciprocalSqrt()
}

// LLVM-AMD64-DAG: call <16 x float> @llvm.x86.avx512.rcp14.ps.512({{.*}}i16 -1)
// LLVM-ASM-AMD64-DAG: VRCP14PS {{.*Z[0-9]}}

// LLVM-AMD64-DAG: call <16 x float> @llvm.x86.avx512.rsqrt14.ps.512({{.*}}i16 -1)
// LLVM-ASM-AMD64-DAG: VRSQRT14PS {{.*Z[0-9]}}
//
//go:noinline
func recip32x16(x archsimd.Float32x16) (archsimd.Float32x16, archsimd.Float32x16) {
	if !archsimd.X86.AVX512() {
		return x, x
	}
	return x.Reciprocal(), x.ReciprocalSqrt()
}

// LLVM-AMD64-DAG: call <2 x double> @llvm.x86.avx512.rcp14.pd.128({{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VRCP14PD {{.*X[0-9]}}

// LLVM-AMD64-DAG: call <2 x double> @llvm.x86.avx512.rsqrt14.pd.128({{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VRSQRT14PD {{.*X[0-9]}}
// LLVM-AMD64-DAG: !{!"x86.avx512"}
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.recip64x2<goallc.fmv.avx512>"({{.*}}) [[RECIPATTR:#[0-9]+]]
// LLVM-OPT-AMD64-DAG: attributes [[RECIPATTR]] = { {{.*}}"target-features"="{{[^"]*}}+avx512f{{[^"]*}}"
//
//go:noinline
func recip64x2(x archsimd.Float64x2) (archsimd.Float64x2, archsimd.Float64x2) {
	if !archsimd.X86.AVX512() {
		return x, x
	}
	return x.Reciprocal(), x.ReciprocalSqrt()
}

// LLVM-AMD64-DAG: call <4 x double> @llvm.x86.avx512.rcp14.pd.256({{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VRCP14PD {{.*Y[0-9]}}

// LLVM-AMD64-DAG: call <4 x double> @llvm.x86.avx512.rsqrt14.pd.256({{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VRSQRT14PD {{.*Y[0-9]}}
//
//go:noinline
func recip64x4(x archsimd.Float64x4) (archsimd.Float64x4, archsimd.Float64x4) {
	if !archsimd.X86.AVX512() {
		return x, x
	}
	return x.Reciprocal(), x.ReciprocalSqrt()
}

// LLVM-AMD64-DAG: call <8 x double> @llvm.x86.avx512.rcp14.pd.512({{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VRCP14PD {{.*Z[0-9]}}

// LLVM-AMD64-DAG: call <8 x double> @llvm.x86.avx512.rsqrt14.pd.512({{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VRSQRT14PD {{.*Z[0-9]}}
//
//go:noinline
func recip64x8(x archsimd.Float64x8) (archsimd.Float64x8, archsimd.Float64x8) {
	if !archsimd.X86.AVX512() {
		return x, x
	}
	return x.Reciprocal(), x.ReciprocalSqrt()
}
