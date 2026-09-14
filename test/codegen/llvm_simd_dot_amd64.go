// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-AMD64-DAG: define {{.*}} @codegen.dotPairs128(
// LLVM-AMD64-DAG: mul <8 x i32>
// LLVM-AMD64-DAG: add <4 x i32>
// LLVM-ASM-AMD64-DAG: VPMADDWD
//
//go:noinline
func dotPairs128(x, y archsimd.Int16x8) archsimd.Int32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x4{}
	}
	return x.DotProductPairs(y)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.dotPairsSaturated256(
// LLVM-AMD64-DAG: call <16 x i16> @llvm.x86.avx2.pmadd.ub.sw
// LLVM-ASM-AMD64-DAG: VPMADDUBSW
//
//go:noinline
func dotPairsSaturated256(x archsimd.Uint8x32, y archsimd.Int8x32) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return x.DotProductPairsSaturated(y)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.sum8AbsDiff512(
// LLVM-AMD64-DAG: call <8 x i64> @llvm.x86.avx512.psad.bw.512
// LLVM-ASM-AMD64-DAG: VPSADBW
//
//go:noinline
func sum8AbsDiff512(x, y archsimd.Uint8x64) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return x.SumOf8AbsDiff(y)
}
