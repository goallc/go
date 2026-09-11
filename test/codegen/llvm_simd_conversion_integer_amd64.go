// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && amd64

package codegen

import (
	"simd/archsimd"
)

// Shape-changing conversions retain the existing early CPU-feature FMV
// boundary. The 256-bit result ABI needs AVX; widening additionally needs AVX2.
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmExtendToInt16Int8x16<goallc.fmv.baseline>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmExtendToInt16Int8x16<goallc.fmv.avx2>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmExtendToInt16Int8x16<goallc.fmv.resolve>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmTruncToUint8Uint64x2<goallc.fmv.baseline>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmTruncToUint8Uint64x2<goallc.fmv.avx512>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmTruncToUint8Uint64x2<goallc.fmv.resolve>"
// LLVM-ASM-AMD64-DAG: VPMOVZXBW
// LLVM-ASM-AMD64-DAG: VPMOVWB
// LLVM-ASM-AMD64-DAG: VPMOVQB
// LLVM-ASM-AMD64-DAG: VPMOVSXBQ
// LLVM-NM-AMD64: codegen.llvmExtendToInt16Int8x16.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-3: codegen.llvmExtendToInt16Int8x16<1>

// LLVM-AMD64-DAG: sext <16 x i8> {{.*}} to <16 x i16>
//
//go:noinline
func llvmExtendToInt16Int8x16(x archsimd.Int8x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return x.ExtendToInt16()
}

// LLVM-AMD64-DAG: sext <4 x i8> {{.*}} to <4 x i64>
//
//go:noinline
func llvmExtendLo4ToInt64Int8x16(x archsimd.Int8x16) archsimd.Int64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x4{}
	}
	return x.ExtendLo4ToInt64()
}

// LLVM-AMD64-DAG: zext <32 x i8> {{.*}} to <32 x i16>
//
//go:noinline
func llvmExtendToUint16Uint8x32(x archsimd.Uint8x32) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return x.ExtendToUint16()
}

// LLVM-AMD64-DAG: trunc <32 x i16> {{.*}} to <32 x i8>
//
//go:noinline
func llvmTruncToInt8Int16x32(x archsimd.Int16x32) archsimd.Int8x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x32{}
	}
	return x.TruncToInt8()
}

// LLVM-AMD64-DAG: trunc <2 x i64> {{.*}} to <2 x i8>
// LLVM-AMD64-DAG: shufflevector <2 x i8> {{.*}}, <2 x i8> zeroinitializer
//
//go:noinline
func llvmTruncToUint8Uint64x2(x archsimd.Uint64x2) archsimd.Uint8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x16{}
	}
	return x.TruncToUint8()
}
