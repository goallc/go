// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-AMD64-DAG: define {{.*}} @codegen.horizontalInt16x8ConcatAddPairs(
// LLVM-AMD64-DAG: add <8 x i16>
// LLVM-ASM-AMD64-DAG: VPHADDW
//
//go:noinline
func horizontalInt16x8ConcatAddPairs(x, y archsimd.Int16x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Int16x8{}
	}
	return x.ConcatAddPairs(y)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.horizontalInt32x8ConcatSubPairsGrouped(
// LLVM-AMD64-DAG: sub <8 x i32>
// LLVM-ASM-AMD64-DAG: VPHSUBD
//
//go:noinline
func horizontalInt32x8ConcatSubPairsGrouped(x, y archsimd.Int32x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return x.ConcatSubPairsGrouped(y)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.horizontalInt16x16ConcatAddPairsSaturatedGrouped(
// LLVM-AMD64-DAG: call <16 x i16> @llvm.sadd.sat.v16i16
// LLVM-ASM-AMD64-DAG: VPHADDSW
//
//go:noinline
func horizontalInt16x16ConcatAddPairsSaturatedGrouped(x, y archsimd.Int16x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return x.ConcatAddPairsSaturatedGrouped(y)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.horizontalInt16x8ConcatSubPairsSaturated(
// LLVM-AMD64-DAG: call <8 x i16> @llvm.ssub.sat.v8i16
// LLVM-ASM-AMD64-DAG: VPHSUBSW
//
//go:noinline
func horizontalInt16x8ConcatSubPairsSaturated(x, y archsimd.Int16x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Int16x8{}
	}
	return x.ConcatSubPairsSaturated(y)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.horizontalFloat32x8ConcatAddPairsGrouped(
// LLVM-AMD64-DAG: fadd <8 x float>
// LLVM-ASM-AMD64-DAG: VHADDPS
//
//go:noinline
func horizontalFloat32x8ConcatAddPairsGrouped(x, y archsimd.Float32x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x8{}
	}
	return x.ConcatAddPairsGrouped(y)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.horizontalFloat64x4AddOddSubEven(
// LLVM-AMD64-DAG: fsub <4 x double>
// LLVM-ASM-AMD64-DAG: VADDSUBPD
//
//go:noinline
func horizontalFloat64x4AddOddSubEven(x, y archsimd.Float64x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x4{}
	}
	return x.AddOddSubEven(y)
}
