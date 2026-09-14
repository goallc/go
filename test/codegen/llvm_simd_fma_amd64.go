// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-AMD64-DAG: define {{.*}} @codegen.fmaFloat32x4(
// LLVM-AMD64-DAG: call <4 x float> @llvm.fma.v4f32
// LLVM-OPT-AMD64-DAG: call <4 x float> @llvm.fma.v4f32
//
//go:noinline
func fmaFloat32x4(x, y, z archsimd.Float32x4) archsimd.Float32x4 {
	if !archsimd.X86.FMA() {
		return archsimd.Float32x4{}
	}
	return x.MulAdd(y, z)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.fmaFloat32x8(
// LLVM-AMD64-DAG: call <8 x float> @llvm.fma.v8f32
// LLVM-OPT-AMD64-DAG: call <8 x float> @llvm.fma.v8f32
//
//go:noinline
func fmaFloat32x8(x, y, z archsimd.Float32x8) archsimd.Float32x8 {
	if !archsimd.X86.FMA() {
		return archsimd.Float32x8{}
	}
	return x.MulAddEvenSubOdd(y, z)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.fmaFloat32x16(
// LLVM-AMD64-DAG: call <16 x float> @llvm.fma.v16f32
// LLVM-OPT-AMD64-DAG: call <16 x float> @llvm.fma.v16f32
//
//go:noinline
func fmaFloat32x16(x, y, z archsimd.Float32x16) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return x.MulAddOddSubEven(y, z)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.fmaFloat64x2(
// LLVM-AMD64-DAG: call <2 x double> @llvm.fma.v2f64
// LLVM-OPT-AMD64-DAG: call <2 x double> @llvm.fma.v2f64
//
//go:noinline
func fmaFloat64x2(x, y, z archsimd.Float64x2) archsimd.Float64x2 {
	if !archsimd.X86.FMA() {
		return archsimd.Float64x2{}
	}
	return x.MulAdd(y, z)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.fmaFloat64x4(
// LLVM-AMD64-DAG: call <4 x double> @llvm.fma.v4f64
// LLVM-OPT-AMD64-DAG: call <4 x double> @llvm.fma.v4f64
//
//go:noinline
func fmaFloat64x4(x, y, z archsimd.Float64x4) archsimd.Float64x4 {
	if !archsimd.X86.FMA() {
		return archsimd.Float64x4{}
	}
	return x.MulAddEvenSubOdd(y, z)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.fmaFloat64x8(
// LLVM-AMD64-DAG: call <8 x double> @llvm.fma.v8f64
// LLVM-OPT-AMD64-DAG: call <8 x double> @llvm.fma.v8f64
//
//go:noinline
func fmaFloat64x8(x, y, z archsimd.Float64x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.MulAddOddSubEven(y, z)
}

// LLVM-OPT-AMD64-DAG: <goallc.fmv.fma>
// The 512-bit vector ABI already supplies the AVX512 entry feature floor.
// LLVM-OPT-AMD64-DAG: "target-features"="{{[^"]*}}+avx512f{{[^"]*}}"
// LLVM-ASM-AMD64-DAG: VFMADD
