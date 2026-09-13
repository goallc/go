// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-AMD64-DAG: define {{.*}} @codegen.widenInt32x4(
// LLVM-AMD64-DAG: shufflevector <4 x i32>
// LLVM-AMD64-DAG: sext <2 x i32> {{.*}} to <2 x i64>
// LLVM-AMD64-DAG: mul <2 x i64>
//
//go:noinline
func widenInt32x4(x, y archsimd.Int32x4) archsimd.Int64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Int64x2{}
	}
	return x.MulWidenEven(y)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.widenInt32x8(
// LLVM-AMD64-DAG: shufflevector <8 x i32>
// LLVM-AMD64-DAG: sext <4 x i32> {{.*}} to <4 x i64>
// LLVM-AMD64-DAG: mul <4 x i64>
//
//go:noinline
func widenInt32x8(x, y archsimd.Int32x8) archsimd.Int64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x4{}
	}
	return x.MulWidenEven(y)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.widenUint32x4(
// LLVM-AMD64-DAG: shufflevector <4 x i32>
// LLVM-AMD64-DAG: zext <2 x i32> {{.*}} to <2 x i64>
// LLVM-AMD64-DAG: mul <2 x i64>
//
//go:noinline
func widenUint32x4(x, y archsimd.Uint32x4) archsimd.Uint64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint64x2{}
	}
	return x.MulWidenEven(y)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.widenUint32x8(
// LLVM-AMD64-DAG: shufflevector <8 x i32>
// LLVM-AMD64-DAG: zext <4 x i32> {{.*}} to <4 x i64>
// LLVM-AMD64-DAG: mul <4 x i64>
//
//go:noinline
func widenUint32x8(x, y archsimd.Uint32x8) archsimd.Uint64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x4{}
	}
	return x.MulWidenEven(y)
}

// LLVM-OPT-AMD64-DAG: <goallc.fmv.avx2>
// LLVM-ASM-AMD64-DAG: VPMULDQ
// LLVM-ASM-AMD64-DAG: VPMULUDQ
