// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && amd64

package codegen

import (
	"simd/archsimd"
)

// LLVM-ASM-AMD64-DAG: VPMOVSQB Z0, X0
// LLVM-ASM-AMD64-DAG: VPMOVSDW Z1, Y0
// LLVM-ASM-AMD64-DAG: VPACKSSDW X1, X0, X0
// LLVM-ASM-AMD64-DAG: VPACKSSDW Y1, Y0, Y0
// LLVM-ASM-AMD64-DAG: VPACKSSDW Z1, Z0, Z0
// LLVM-ASM-AMD64-DAG: VPMOVUSWB Z1, Y0
// LLVM-ASM-AMD64-DAG: VPMOVUSQB X0, X0
// LLVM-ASM-AMD64-DAG: VPMOVUSQB Z0, X0
// LLVM-ASM-AMD64-DAG: VPACKUSDW X1, X0, X0
// LLVM-ASM-AMD64-DAG: VPACKUSDW Y1, Y0, Y0
// LLVM-ASM-AMD64-DAG: VPACKUSDW Z1, Z0, Z0

// Narrow input/result ABIs keep baseline-safe dispatch and specialize only
// the actual AVX2/AVX512 instruction requirement.
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmSaturateToInt16ConcatGroupedInt32x8<goallc.fmv.baseline>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmSaturateToInt16ConcatGroupedInt32x8<goallc.fmv.avx2>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmSaturateToInt16ConcatGroupedInt32x8<goallc.fmv.resolve>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmSaturateToUint8Uint64x2<goallc.fmv.baseline>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmSaturateToUint8Uint64x2<goallc.fmv.avx512>"
// LLVM-NM-AMD64: codegen.llvmSaturateToInt16ConcatGroupedInt32x8.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-3: codegen.llvmSaturateToInt16ConcatGroupedInt32x8<1>

// LLVM-AMD64-DAG: call <8 x i64> @llvm.smax.v8i64({{.*}}i64 -128
// LLVM-AMD64-DAG: call <8 x i64> @llvm.smin.v8i64({{.*}}i64 127
// LLVM-AMD64-DAG: trunc <8 x i64> {{.*}} to <8 x i8>
// LLVM-AMD64-DAG: shufflevector <8 x i8> {{.*}}, <8 x i8> zeroinitializer
//
//go:noinline
func llvmSaturateToInt8Int64x8(x archsimd.Int64x8) archsimd.Int8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x16{}
	}
	return x.SaturateToInt8()
}

// LLVM-AMD64-DAG: call <16 x i32> @llvm.smax.v16i32({{.*}}i32 -32768
// LLVM-AMD64-DAG: call <16 x i32> @llvm.smin.v16i32({{.*}}i32 32767
// LLVM-AMD64-DAG: trunc <16 x i32> {{.*}} to <16 x i16>
//
//go:noinline
func llvmSaturateToInt16Int32x16(x archsimd.Int32x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x16{}
	}
	return x.SaturateToInt16()
}

// LLVM-AMD64-DAG: call <4 x i32> @llvm.smax.v4i32({{.*}}i32 -32768
// LLVM-AMD64-DAG: call <4 x i32> @llvm.smin.v4i32({{.*}}i32 32767
// LLVM-AMD64-DAG: trunc <4 x i32> {{.*}} to <4 x i16>
//
//go:noinline
func llvmSaturateToInt16ConcatInt32x4(x archsimd.Int32x4, y archsimd.Int32x4) archsimd.Int16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Int16x8{}
	}
	return x.SaturateToInt16Concat(y)
}

// LLVM-AMD64-DAG: call <8 x i32> @llvm.smax.v8i32({{.*}}i32 -32768
// LLVM-AMD64-DAG: call <8 x i32> @llvm.smin.v8i32({{.*}}i32 32767
// LLVM-AMD64-DAG: trunc <8 x i32> {{.*}} to <8 x i16>
// LLVM-AMD64-DAG: <i32 0, i32 1, i32 2, i32 3, i32 8, i32 9, i32 10, i32 11, i32 4, i32 5, i32 6, i32 7, i32 12, i32 13, i32 14, i32 15>
//
//go:noinline
func llvmSaturateToInt16ConcatGroupedInt32x8(x archsimd.Int32x8, y archsimd.Int32x8) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return x.SaturateToInt16ConcatGrouped(y)
}

// LLVM-AMD64-DAG: call <16 x i32> @llvm.smax.v16i32({{.*}}i32 -32768
// LLVM-AMD64-DAG: call <16 x i32> @llvm.smin.v16i32({{.*}}i32 32767
// LLVM-AMD64-DAG: trunc <16 x i32> {{.*}} to <16 x i16>
//
//go:noinline
func llvmSaturateToInt16ConcatGroupedInt32x16(x archsimd.Int32x16, y archsimd.Int32x16) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return x.SaturateToInt16ConcatGrouped(y)
}

// LLVM-AMD64-DAG: call <32 x i16> @llvm.umin.v32i16({{.*}}i16 255
// LLVM-AMD64-DAG: trunc <32 x i16> {{.*}} to <32 x i8>
//
//go:noinline
func llvmSaturateToUint8Uint16x32(x archsimd.Uint16x32) archsimd.Uint8x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x32{}
	}
	return x.SaturateToUint8()
}

// LLVM-AMD64-DAG: call <2 x i64> @llvm.umin.v2i64({{.*}}i64 255
// LLVM-AMD64-DAG: trunc <2 x i64> {{.*}} to <2 x i8>
// LLVM-AMD64-DAG: shufflevector <2 x i8> {{.*}}, <2 x i8> zeroinitializer
//
//go:noinline
func llvmSaturateToUint8Uint64x2(x archsimd.Uint64x2) archsimd.Uint8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x16{}
	}
	return x.SaturateToUint8()
}

// LLVM-AMD64-DAG: call <8 x i64> @llvm.umin.v8i64({{.*}}i64 255
// LLVM-AMD64-DAG: trunc <8 x i64> {{.*}} to <8 x i8>
// LLVM-AMD64-DAG: shufflevector <8 x i8> {{.*}}, <8 x i8> zeroinitializer
//
//go:noinline
func llvmSaturateToUint8Uint64x8(x archsimd.Uint64x8) archsimd.Uint8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x16{}
	}
	return x.SaturateToUint8()
}

// LLVM-AMD64-DAG: call <4 x i32> @llvm.smax.v4i32({{.*}}zeroinitializer
// LLVM-AMD64-DAG: call <4 x i32> @llvm.smin.v4i32({{.*}}i32 65535
// LLVM-AMD64-DAG: trunc <4 x i32> {{.*}} to <4 x i16>
//
//go:noinline
func llvmSaturateToUint16ConcatInt32x4(x archsimd.Int32x4, y archsimd.Int32x4) archsimd.Uint16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint16x8{}
	}
	return x.SaturateToUint16Concat(y)
}

// LLVM-AMD64-DAG: call <8 x i32> @llvm.smax.v8i32({{.*}}zeroinitializer
// LLVM-AMD64-DAG: call <8 x i32> @llvm.smin.v8i32({{.*}}i32 65535
// LLVM-AMD64-DAG: trunc <8 x i32> {{.*}} to <8 x i16>
// LLVM-AMD64-DAG: <i32 0, i32 1, i32 2, i32 3, i32 8, i32 9, i32 10, i32 11, i32 4, i32 5, i32 6, i32 7, i32 12, i32 13, i32 14, i32 15>
//
//go:noinline
func llvmSaturateToUint16ConcatGroupedInt32x8(x archsimd.Int32x8, y archsimd.Int32x8) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return x.SaturateToUint16ConcatGrouped(y)
}

// LLVM-AMD64-DAG: call <16 x i32> @llvm.smax.v16i32({{.*}}zeroinitializer
// LLVM-AMD64-DAG: call <16 x i32> @llvm.smin.v16i32({{.*}}i32 65535
// LLVM-AMD64-DAG: trunc <16 x i32> {{.*}} to <16 x i16>
//
//go:noinline
func llvmSaturateToUint16ConcatGroupedInt32x16(x archsimd.Int32x16, y archsimd.Int32x16) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return x.SaturateToUint16ConcatGrouped(y)
}
