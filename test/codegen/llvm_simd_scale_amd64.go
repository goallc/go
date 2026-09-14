// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-AMD64-DAG: call <4 x float> @llvm.x86.avx512.mask.scalef.ps.128({{.*}}, i8 -1)
// LLVM-ASM-AMD64-DAG: VSCALEFPS {{.*X[0-9]}}
// LLVM-AMD64-DAG: !{!"x86.avx512"}
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.scale32x4<goallc.fmv.avx512>"({{.*}}) [[SCALEATTR:#[0-9]+]]
// LLVM-OPT-AMD64-DAG: attributes [[SCALEATTR]] = { {{.*}}"target-features"="{{[^"]*}}+avx512f{{[^"]*}}"
//
//go:noinline
func scale32x4(x, y archsimd.Float32x4) archsimd.Float32x4 {
	if !archsimd.X86.AVX512() {
		return x
	}
	return x.Scale(y)
}

// LLVM-AMD64-DAG: call <8 x float> @llvm.x86.avx512.mask.scalef.ps.256({{.*}}, i8 -1)
// LLVM-ASM-AMD64-DAG: VSCALEFPS {{.*Y[0-9]}}
//
//go:noinline
func scale32x8(x, y archsimd.Float32x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX512() {
		return x
	}
	return x.Scale(y)
}

// LLVM-AMD64-DAG: call <16 x float> @llvm.x86.avx512.mask.scalef.ps.512({{.*}}, i16 -1, i32 4)
// LLVM-ASM-AMD64-DAG: VSCALEFPS {{.*Z[0-9]}}
//
//go:noinline
func scale32x16(x, y archsimd.Float32x16) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return x
	}
	return x.Scale(y)
}

// LLVM-AMD64-DAG: call <2 x double> @llvm.x86.avx512.mask.scalef.pd.128({{.*}}, i8 -1)
// LLVM-ASM-AMD64-DAG: VSCALEFPD {{.*X[0-9]}}
//
//go:noinline
func scale64x2(x, y archsimd.Float64x2) archsimd.Float64x2 {
	if !archsimd.X86.AVX512() {
		return x
	}
	return x.Scale(y)
}

// LLVM-AMD64-DAG: call <4 x double> @llvm.x86.avx512.mask.scalef.pd.256({{.*}}, i8 -1)
// LLVM-ASM-AMD64-DAG: VSCALEFPD {{.*Y[0-9]}}
//
//go:noinline
func scale64x4(x, y archsimd.Float64x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX512() {
		return x
	}
	return x.Scale(y)
}

// LLVM-AMD64-DAG: call <8 x double> @llvm.x86.avx512.mask.scalef.pd.512({{.*}}, i8 -1, i32 4)
// LLVM-ASM-AMD64-DAG: VSCALEFPD {{.*Z[0-9]}}
//
//go:noinline
func scale64x8(x, y archsimd.Float64x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return x
	}
	return x.Scale(y)
}
