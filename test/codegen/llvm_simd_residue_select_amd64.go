// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-AMD64-DAG: call <4 x float> @llvm.x86.avx512.mask.reduce.ps.128({{.*}}i32 240, {{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VREDUCEPS $0xf0, {{.*X[0-9]}}
// LLVM-AMD64-DAG: call <4 x float> @llvm.x86.avx512.mask.reduce.ps.128({{.*}}i32 241, {{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VREDUCEPS $0xf1, {{.*X[0-9]}}
// LLVM-AMD64-DAG: call <4 x float> @llvm.x86.avx512.mask.reduce.ps.128({{.*}}i32 242, {{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VREDUCEPS $0xf2, {{.*X[0-9]}}
// LLVM-AMD64-DAG: call <4 x float> @llvm.x86.avx512.mask.reduce.ps.128({{.*}}i32 243, {{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VREDUCEPS $0xf3, {{.*X[0-9]}}
//
//go:noinline
func residue32x4(x archsimd.Float32x4) (archsimd.Float32x4, archsimd.Float32x4, archsimd.Float32x4, archsimd.Float32x4) {
	if !archsimd.X86.AVX512() {
		return x, x, x, x
	}
	return x.RoundScaledResidue(255), x.FloorScaledResidue(255), x.CeilScaledResidue(255), x.TruncScaledResidue(255)
}

// LLVM-AMD64-DAG: call <8 x float> @llvm.x86.avx512.mask.reduce.ps.256({{.*}}i32 240, {{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VREDUCEPS $0xf0, {{.*Y[0-9]}}
// LLVM-AMD64-DAG: call <8 x float> @llvm.x86.avx512.mask.reduce.ps.256({{.*}}i32 241, {{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VREDUCEPS $0xf1, {{.*Y[0-9]}}
// LLVM-AMD64-DAG: call <8 x float> @llvm.x86.avx512.mask.reduce.ps.256({{.*}}i32 242, {{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VREDUCEPS $0xf2, {{.*Y[0-9]}}
// LLVM-AMD64-DAG: call <8 x float> @llvm.x86.avx512.mask.reduce.ps.256({{.*}}i32 243, {{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VREDUCEPS $0xf3, {{.*Y[0-9]}}
//
//go:noinline
func residue32x8(x archsimd.Float32x8) (archsimd.Float32x8, archsimd.Float32x8, archsimd.Float32x8, archsimd.Float32x8) {
	if !archsimd.X86.AVX512() {
		return x, x, x, x
	}
	return x.RoundScaledResidue(255), x.FloorScaledResidue(255), x.CeilScaledResidue(255), x.TruncScaledResidue(255)
}

// LLVM-AMD64-DAG: call <16 x float> @llvm.x86.avx512.mask.reduce.ps.512({{.*}}i32 240, {{.*}}i16 -1, i32 4)
// LLVM-ASM-AMD64-DAG: VREDUCEPS $0xf0, {{.*Z[0-9]}}
// LLVM-AMD64-DAG: call <16 x float> @llvm.x86.avx512.mask.reduce.ps.512({{.*}}i32 241, {{.*}}i16 -1, i32 4)
// LLVM-ASM-AMD64-DAG: VREDUCEPS $0xf1, {{.*Z[0-9]}}
// LLVM-AMD64-DAG: call <16 x float> @llvm.x86.avx512.mask.reduce.ps.512({{.*}}i32 242, {{.*}}i16 -1, i32 4)
// LLVM-ASM-AMD64-DAG: VREDUCEPS $0xf2, {{.*Z[0-9]}}
// LLVM-AMD64-DAG: call <16 x float> @llvm.x86.avx512.mask.reduce.ps.512({{.*}}i32 243, {{.*}}i16 -1, i32 4)
// LLVM-ASM-AMD64-DAG: VREDUCEPS $0xf3, {{.*Z[0-9]}}
//
//go:noinline
func residue32x16(x archsimd.Float32x16) (archsimd.Float32x16, archsimd.Float32x16, archsimd.Float32x16, archsimd.Float32x16) {
	if !archsimd.X86.AVX512() {
		return x, x, x, x
	}
	return x.RoundScaledResidue(255), x.FloorScaledResidue(255), x.CeilScaledResidue(255), x.TruncScaledResidue(255)
}

// LLVM-AMD64-DAG: call <2 x double> @llvm.x86.avx512.mask.reduce.pd.128({{.*}}i32 240, {{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VREDUCEPD $0xf0, {{.*X[0-9]}}
// LLVM-AMD64-DAG: call <2 x double> @llvm.x86.avx512.mask.reduce.pd.128({{.*}}i32 241, {{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VREDUCEPD $0xf1, {{.*X[0-9]}}
// LLVM-AMD64-DAG: call <2 x double> @llvm.x86.avx512.mask.reduce.pd.128({{.*}}i32 242, {{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VREDUCEPD $0xf2, {{.*X[0-9]}}
// LLVM-AMD64-DAG: call <2 x double> @llvm.x86.avx512.mask.reduce.pd.128({{.*}}i32 243, {{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VREDUCEPD $0xf3, {{.*X[0-9]}}
// LLVM-AMD64-DAG: !{!"x86.avx512"}
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.residue64x2<goallc.fmv.avx512>"({{.*}}) [[RESIDUEATTR:#[0-9]+]]
// LLVM-OPT-AMD64-DAG: attributes [[RESIDUEATTR]] = { {{.*}}"target-features"="{{[^"]*}}+avx512dq{{[^"]*}}"
//
//go:noinline
func residue64x2(x archsimd.Float64x2) (archsimd.Float64x2, archsimd.Float64x2, archsimd.Float64x2, archsimd.Float64x2) {
	if !archsimd.X86.AVX512() {
		return x, x, x, x
	}
	return x.RoundScaledResidue(255), x.FloorScaledResidue(255), x.CeilScaledResidue(255), x.TruncScaledResidue(255)
}

// LLVM-AMD64-DAG: call <4 x double> @llvm.x86.avx512.mask.reduce.pd.256({{.*}}i32 240, {{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VREDUCEPD $0xf0, {{.*Y[0-9]}}
// LLVM-AMD64-DAG: call <4 x double> @llvm.x86.avx512.mask.reduce.pd.256({{.*}}i32 241, {{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VREDUCEPD $0xf1, {{.*Y[0-9]}}
// LLVM-AMD64-DAG: call <4 x double> @llvm.x86.avx512.mask.reduce.pd.256({{.*}}i32 242, {{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VREDUCEPD $0xf2, {{.*Y[0-9]}}
// LLVM-AMD64-DAG: call <4 x double> @llvm.x86.avx512.mask.reduce.pd.256({{.*}}i32 243, {{.*}}i8 -1)
// LLVM-ASM-AMD64-DAG: VREDUCEPD $0xf3, {{.*Y[0-9]}}
//
//go:noinline
func residue64x4(x archsimd.Float64x4) (archsimd.Float64x4, archsimd.Float64x4, archsimd.Float64x4, archsimd.Float64x4) {
	if !archsimd.X86.AVX512() {
		return x, x, x, x
	}
	return x.RoundScaledResidue(255), x.FloorScaledResidue(255), x.CeilScaledResidue(255), x.TruncScaledResidue(255)
}

// LLVM-AMD64-DAG: call <8 x double> @llvm.x86.avx512.mask.reduce.pd.512({{.*}}i32 240, {{.*}}i8 -1, i32 4)
// LLVM-ASM-AMD64-DAG: VREDUCEPD $0xf0, {{.*Z[0-9]}}
// LLVM-AMD64-DAG: call <8 x double> @llvm.x86.avx512.mask.reduce.pd.512({{.*}}i32 241, {{.*}}i8 -1, i32 4)
// LLVM-ASM-AMD64-DAG: VREDUCEPD $0xf1, {{.*Z[0-9]}}
// LLVM-AMD64-DAG: call <8 x double> @llvm.x86.avx512.mask.reduce.pd.512({{.*}}i32 242, {{.*}}i8 -1, i32 4)
// LLVM-ASM-AMD64-DAG: VREDUCEPD $0xf2, {{.*Z[0-9]}}
// LLVM-AMD64-DAG: call <8 x double> @llvm.x86.avx512.mask.reduce.pd.512({{.*}}i32 243, {{.*}}i8 -1, i32 4)
// LLVM-ASM-AMD64-DAG: VREDUCEPD $0xf3, {{.*Z[0-9]}}
//
//go:noinline
func residue64x8(x archsimd.Float64x8) (archsimd.Float64x8, archsimd.Float64x8, archsimd.Float64x8, archsimd.Float64x8) {
	if !archsimd.X86.AVX512() {
		return x, x, x, x
	}
	return x.RoundScaledResidue(255), x.FloorScaledResidue(255), x.CeilScaledResidue(255), x.TruncScaledResidue(255)
}

// LLVM-AMD64-DAG: icmp sgt <32 x i8>
// LLVM-AMD64-DAG: select <32 x i1>
// LLVM-ASM-AMD64-DAG: VPMAXSB {{.*Y[0-9]}}
//
//go:noinline
func selectBytes(x, y archsimd.Int8x32) archsimd.Int8x32 {
	if !archsimd.X86.AVX2() {
		return x
	}
	return x.IfElse(x.Greater(y), y)
}
