// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && amd64

package codegen

import "simd/archsimd"

// LLVM-ASM-AMD64-DAG: VEXTRACTF128 $0x1, Y1, X0
// LLVM-ASM-AMD64-DAG: VEXTRACTF64X4 $0x1, Z1, Y0
// LLVM-ASM-AMD64-DAG: VINSERTF128 $0x1, X1, Y2, Y0
// LLVM-ASM-AMD64-DAG: VINSERTF64X4 $0x1, Y1, Z2, Z0
// LLVM-ASM-AMD64-DAG: VINSERTF64X4 $0x0, Y1, Z2, Z0
// LLVM-ASM-AMD64-DAG: VPUNPCKHWD X1, X2, X0
// LLVM-ASM-AMD64-DAG: VPUNPCKHWD Y1, Y0, Y0
// LLVM-ASM-AMD64-DAG: VPUNPCKHWD Z1, Z2, Z0
// LLVM-ASM-AMD64-DAG: VPUNPCKLWD X1, X2, X0
// LLVM-ASM-AMD64-DAG: VPUNPCKLWD Y1, Y0, Y0
// LLVM-ASM-AMD64-DAG: VPUNPCKLWD Z1, Z2, Z0
// LLVM-ASM-AMD64-DAG: VBROADCASTSS X0, X0
// LLVM-ASM-AMD64-DAG: VBROADCASTSS X0, Y0
// LLVM-ASM-AMD64-DAG: VBROADCASTSS X1, Z0
// LLVM-ASM-AMD64-DAG: VPBROADCASTB X0, X0
// LLVM-ASM-AMD64-DAG: VPBROADCASTB X0, Y0
// LLVM-ASM-AMD64-DAG: VPBROADCASTB X1, Z0
// LLVM-ASM-AMD64-DAG: VPBROADCASTW X0, X0
// LLVM-ASM-AMD64-DAG: VPBROADCASTW X0, Y0
// LLVM-ASM-AMD64-DAG: VPBROADCASTW X1, Z0
// LLVM-ASM-AMD64-DAG: VBROADCASTSD X0, Y0
// LLVM-ASM-AMD64-DAG: VBROADCASTSD X1, Z0

// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmShuffleGetHiInt32x8<goallc.fmv.baseline>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmShuffleGetHiInt32x8<goallc.fmv.avx2>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmShuffleGetHiInt32x8<goallc.fmv.resolve>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmShuffleSetHiInt32x8<goallc.fmv.baseline>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmShuffleSetHiInt32x8<goallc.fmv.avx2>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmShuffleSetHiInt32x8<goallc.fmv.resolve>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmShufflebroadcast1To4Int32x4<goallc.fmv.baseline>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmShufflebroadcast1To4Int32x4<goallc.fmv.avx2>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmShufflebroadcast1To4Int32x4<goallc.fmv.resolve>"

// LLVM-NM-AMD64: codegen.llvmShuffleGetHiInt32x8.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-3: codegen.llvmShuffleGetHiInt32x8<1>
// LLVM-NM-AMD64: codegen.llvmShuffleSetHiInt32x8.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-3: codegen.llvmShuffleSetHiInt32x8<1>
// LLVM-NM-AMD64: codegen.llvmShufflebroadcast1To4Int32x4.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-3: codegen.llvmShufflebroadcast1To4Int32x4<1>

// Unique parameter names tie each routing check to its API without depending
// on function emission order under parallel compilation.
// LLVM-AMD64-DAG: define {{.*}} <16 x float> @codegen.llvmShufflebroadcast1To16Float32x4(<4 x float> %xbroadcast1To16Float32x4
// LLVM-AMD64-DAG: extractelement <4 x float> %xbroadcast1To16Float32x4, i32 0
// LLVM-AMD64-DAG: shufflevector <4 x float> {{%[^,]+}}, <4 x float> zeroinitializer, <16 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <16 x i16> @codegen.llvmShufflebroadcast1To16Int16x8(<8 x i16> %xbroadcast1To16Int16x8
// LLVM-AMD64-DAG: extractelement <8 x i16> %xbroadcast1To16Int16x8, i32 0
// LLVM-AMD64-DAG: shufflevector <8 x i16> {{%[^,]+}}, <8 x i16> zeroinitializer, <16 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <16 x i32> @codegen.llvmShufflebroadcast1To16Int32x4(<4 x i32> %xbroadcast1To16Int32x4
// LLVM-AMD64-DAG: extractelement <4 x i32> %xbroadcast1To16Int32x4, i32 0
// LLVM-AMD64-DAG: shufflevector <4 x i32> {{%[^,]+}}, <4 x i32> zeroinitializer, <16 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <16 x i8> @codegen.llvmShufflebroadcast1To16Int8x16(<16 x i8> %xbroadcast1To16Int8x16
// LLVM-AMD64-DAG: extractelement <16 x i8> %xbroadcast1To16Int8x16, i32 0
// LLVM-AMD64-DAG: shufflevector <16 x i8> {{%[^,]+}}, <16 x i8> zeroinitializer, <16 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <16 x i16> @codegen.llvmShufflebroadcast1To16Uint16x8(<8 x i16> %xbroadcast1To16Uint16x8
// LLVM-AMD64-DAG: extractelement <8 x i16> %xbroadcast1To16Uint16x8, i32 0
// LLVM-AMD64-DAG: shufflevector <8 x i16> {{%[^,]+}}, <8 x i16> zeroinitializer, <16 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <16 x i32> @codegen.llvmShufflebroadcast1To16Uint32x4(<4 x i32> %xbroadcast1To16Uint32x4
// LLVM-AMD64-DAG: extractelement <4 x i32> %xbroadcast1To16Uint32x4, i32 0
// LLVM-AMD64-DAG: shufflevector <4 x i32> {{%[^,]+}}, <4 x i32> zeroinitializer, <16 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <16 x i8> @codegen.llvmShufflebroadcast1To16Uint8x16(<16 x i8> %xbroadcast1To16Uint8x16
// LLVM-AMD64-DAG: extractelement <16 x i8> %xbroadcast1To16Uint8x16, i32 0
// LLVM-AMD64-DAG: shufflevector <16 x i8> {{%[^,]+}}, <16 x i8> zeroinitializer, <16 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <2 x double> @codegen.llvmShufflebroadcast1To2Float64x2(<2 x double> %xbroadcast1To2Float64x2
// LLVM-AMD64-DAG: extractelement <2 x double> %xbroadcast1To2Float64x2, i32 0
// LLVM-AMD64-DAG: shufflevector <2 x double> {{%[^,]+}}, <2 x double> zeroinitializer, <2 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <2 x i64> @codegen.llvmShufflebroadcast1To2Int64x2(<2 x i64> %xbroadcast1To2Int64x2
// LLVM-AMD64-DAG: extractelement <2 x i64> %xbroadcast1To2Int64x2, i32 0
// LLVM-AMD64-DAG: shufflevector <2 x i64> {{%[^,]+}}, <2 x i64> zeroinitializer, <2 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <2 x i64> @codegen.llvmShufflebroadcast1To2Uint64x2(<2 x i64> %xbroadcast1To2Uint64x2
// LLVM-AMD64-DAG: extractelement <2 x i64> %xbroadcast1To2Uint64x2, i32 0
// LLVM-AMD64-DAG: shufflevector <2 x i64> {{%[^,]+}}, <2 x i64> zeroinitializer, <2 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <32 x i16> @codegen.llvmShufflebroadcast1To32Int16x8(<8 x i16> %xbroadcast1To32Int16x8
// LLVM-AMD64-DAG: extractelement <8 x i16> %xbroadcast1To32Int16x8, i32 0
// LLVM-AMD64-DAG: shufflevector <8 x i16> {{%[^,]+}}, <8 x i16> zeroinitializer, <32 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <32 x i8> @codegen.llvmShufflebroadcast1To32Int8x16(<16 x i8> %xbroadcast1To32Int8x16
// LLVM-AMD64-DAG: extractelement <16 x i8> %xbroadcast1To32Int8x16, i32 0
// LLVM-AMD64-DAG: shufflevector <16 x i8> {{%[^,]+}}, <16 x i8> zeroinitializer, <32 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <32 x i16> @codegen.llvmShufflebroadcast1To32Uint16x8(<8 x i16> %xbroadcast1To32Uint16x8
// LLVM-AMD64-DAG: extractelement <8 x i16> %xbroadcast1To32Uint16x8, i32 0
// LLVM-AMD64-DAG: shufflevector <8 x i16> {{%[^,]+}}, <8 x i16> zeroinitializer, <32 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <32 x i8> @codegen.llvmShufflebroadcast1To32Uint8x16(<16 x i8> %xbroadcast1To32Uint8x16
// LLVM-AMD64-DAG: extractelement <16 x i8> %xbroadcast1To32Uint8x16, i32 0
// LLVM-AMD64-DAG: shufflevector <16 x i8> {{%[^,]+}}, <16 x i8> zeroinitializer, <32 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <4 x float> @codegen.llvmShufflebroadcast1To4Float32x4(<4 x float> %xbroadcast1To4Float32x4
// LLVM-AMD64-DAG: extractelement <4 x float> %xbroadcast1To4Float32x4, i32 0
// LLVM-AMD64-DAG: shufflevector <4 x float> {{%[^,]+}}, <4 x float> zeroinitializer, <4 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <4 x double> @codegen.llvmShufflebroadcast1To4Float64x2(<2 x double> %xbroadcast1To4Float64x2
// LLVM-AMD64-DAG: extractelement <2 x double> %xbroadcast1To4Float64x2, i32 0
// LLVM-AMD64-DAG: shufflevector <2 x double> {{%[^,]+}}, <2 x double> zeroinitializer, <4 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <4 x i32> @codegen.llvmShufflebroadcast1To4Int32x4(<4 x i32> %xbroadcast1To4Int32x4
// LLVM-AMD64-DAG: extractelement <4 x i32> %xbroadcast1To4Int32x4, i32 0
// LLVM-AMD64-DAG: shufflevector <4 x i32> {{%[^,]+}}, <4 x i32> zeroinitializer, <4 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <4 x i64> @codegen.llvmShufflebroadcast1To4Int64x2(<2 x i64> %xbroadcast1To4Int64x2
// LLVM-AMD64-DAG: extractelement <2 x i64> %xbroadcast1To4Int64x2, i32 0
// LLVM-AMD64-DAG: shufflevector <2 x i64> {{%[^,]+}}, <2 x i64> zeroinitializer, <4 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <4 x i32> @codegen.llvmShufflebroadcast1To4Uint32x4(<4 x i32> %xbroadcast1To4Uint32x4
// LLVM-AMD64-DAG: extractelement <4 x i32> %xbroadcast1To4Uint32x4, i32 0
// LLVM-AMD64-DAG: shufflevector <4 x i32> {{%[^,]+}}, <4 x i32> zeroinitializer, <4 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <4 x i64> @codegen.llvmShufflebroadcast1To4Uint64x2(<2 x i64> %xbroadcast1To4Uint64x2
// LLVM-AMD64-DAG: extractelement <2 x i64> %xbroadcast1To4Uint64x2, i32 0
// LLVM-AMD64-DAG: shufflevector <2 x i64> {{%[^,]+}}, <2 x i64> zeroinitializer, <4 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <64 x i8> @codegen.llvmShufflebroadcast1To64Int8x16(<16 x i8> %xbroadcast1To64Int8x16
// LLVM-AMD64-DAG: extractelement <16 x i8> %xbroadcast1To64Int8x16, i32 0
// LLVM-AMD64-DAG: shufflevector <16 x i8> {{%[^,]+}}, <16 x i8> zeroinitializer, <64 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <64 x i8> @codegen.llvmShufflebroadcast1To64Uint8x16(<16 x i8> %xbroadcast1To64Uint8x16
// LLVM-AMD64-DAG: extractelement <16 x i8> %xbroadcast1To64Uint8x16, i32 0
// LLVM-AMD64-DAG: shufflevector <16 x i8> {{%[^,]+}}, <16 x i8> zeroinitializer, <64 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <8 x float> @codegen.llvmShufflebroadcast1To8Float32x4(<4 x float> %xbroadcast1To8Float32x4
// LLVM-AMD64-DAG: extractelement <4 x float> %xbroadcast1To8Float32x4, i32 0
// LLVM-AMD64-DAG: shufflevector <4 x float> {{%[^,]+}}, <4 x float> zeroinitializer, <8 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <8 x double> @codegen.llvmShufflebroadcast1To8Float64x2(<2 x double> %xbroadcast1To8Float64x2
// LLVM-AMD64-DAG: extractelement <2 x double> %xbroadcast1To8Float64x2, i32 0
// LLVM-AMD64-DAG: shufflevector <2 x double> {{%[^,]+}}, <2 x double> zeroinitializer, <8 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <8 x i16> @codegen.llvmShufflebroadcast1To8Int16x8(<8 x i16> %xbroadcast1To8Int16x8
// LLVM-AMD64-DAG: extractelement <8 x i16> %xbroadcast1To8Int16x8, i32 0
// LLVM-AMD64-DAG: shufflevector <8 x i16> {{%[^,]+}}, <8 x i16> zeroinitializer, <8 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <8 x i32> @codegen.llvmShufflebroadcast1To8Int32x4(<4 x i32> %xbroadcast1To8Int32x4
// LLVM-AMD64-DAG: extractelement <4 x i32> %xbroadcast1To8Int32x4, i32 0
// LLVM-AMD64-DAG: shufflevector <4 x i32> {{%[^,]+}}, <4 x i32> zeroinitializer, <8 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <8 x i64> @codegen.llvmShufflebroadcast1To8Int64x2(<2 x i64> %xbroadcast1To8Int64x2
// LLVM-AMD64-DAG: extractelement <2 x i64> %xbroadcast1To8Int64x2, i32 0
// LLVM-AMD64-DAG: shufflevector <2 x i64> {{%[^,]+}}, <2 x i64> zeroinitializer, <8 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <8 x i16> @codegen.llvmShufflebroadcast1To8Uint16x8(<8 x i16> %xbroadcast1To8Uint16x8
// LLVM-AMD64-DAG: extractelement <8 x i16> %xbroadcast1To8Uint16x8, i32 0
// LLVM-AMD64-DAG: shufflevector <8 x i16> {{%[^,]+}}, <8 x i16> zeroinitializer, <8 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <8 x i32> @codegen.llvmShufflebroadcast1To8Uint32x4(<4 x i32> %xbroadcast1To8Uint32x4
// LLVM-AMD64-DAG: extractelement <4 x i32> %xbroadcast1To8Uint32x4, i32 0
// LLVM-AMD64-DAG: shufflevector <4 x i32> {{%[^,]+}}, <4 x i32> zeroinitializer, <8 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <8 x i64> @codegen.llvmShufflebroadcast1To8Uint64x2(<2 x i64> %xbroadcast1To8Uint64x2
// LLVM-AMD64-DAG: extractelement <2 x i64> %xbroadcast1To8Uint64x2, i32 0
// LLVM-AMD64-DAG: shufflevector <2 x i64> {{%[^,]+}}, <2 x i64> zeroinitializer, <8 x i32> zeroinitializer
// LLVM-AMD64-DAG: define {{.*}} <8 x float> @codegen.llvmShuffleGetHiFloat32x16(<16 x float> %xGetHiFloat32x16
// LLVM-AMD64-DAG: shufflevector <16 x float> %xGetHiFloat32x16, <16 x float> zeroinitializer, <8 x i32> <i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <4 x float> @codegen.llvmShuffleGetHiFloat32x8(<8 x float> %xGetHiFloat32x8
// LLVM-AMD64-DAG: shufflevector <8 x float> %xGetHiFloat32x8, <8 x float> zeroinitializer, <4 x i32> <i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <2 x double> @codegen.llvmShuffleGetHiFloat64x4(<4 x double> %xGetHiFloat64x4
// LLVM-AMD64-DAG: shufflevector <4 x double> %xGetHiFloat64x4, <4 x double> zeroinitializer, <2 x i32> <i32 2, i32 3>
// LLVM-AMD64-DAG: define {{.*}} <4 x double> @codegen.llvmShuffleGetHiFloat64x8(<8 x double> %xGetHiFloat64x8
// LLVM-AMD64-DAG: shufflevector <8 x double> %xGetHiFloat64x8, <8 x double> zeroinitializer, <4 x i32> <i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleGetHiInt16x16(<16 x i16> %xGetHiInt16x16
// LLVM-AMD64-DAG: shufflevector <16 x i16> %xGetHiInt16x16, <16 x i16> zeroinitializer, <8 x i32> <i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <16 x i16> @codegen.llvmShuffleGetHiInt16x32(<32 x i16> %xGetHiInt16x32
// LLVM-AMD64-DAG: shufflevector <32 x i16> %xGetHiInt16x32, <32 x i16> zeroinitializer, <16 x i32> <i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>
// LLVM-AMD64-DAG: define {{.*}} <8 x i32> @codegen.llvmShuffleGetHiInt32x16(<16 x i32> %xGetHiInt32x16
// LLVM-AMD64-DAG: shufflevector <16 x i32> %xGetHiInt32x16, <16 x i32> zeroinitializer, <8 x i32> <i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleGetHiInt32x8(<8 x i32> %xGetHiInt32x8
// LLVM-AMD64-DAG: shufflevector <8 x i32> %xGetHiInt32x8, <8 x i32> zeroinitializer, <4 x i32> <i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleGetHiInt64x4(<4 x i64> %xGetHiInt64x4
// LLVM-AMD64-DAG: shufflevector <4 x i64> %xGetHiInt64x4, <4 x i64> zeroinitializer, <2 x i32> <i32 2, i32 3>
// LLVM-AMD64-DAG: define {{.*}} <4 x i64> @codegen.llvmShuffleGetHiInt64x8(<8 x i64> %xGetHiInt64x8
// LLVM-AMD64-DAG: shufflevector <8 x i64> %xGetHiInt64x8, <8 x i64> zeroinitializer, <4 x i32> <i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <16 x i8> @codegen.llvmShuffleGetHiInt8x32(<32 x i8> %xGetHiInt8x32
// LLVM-AMD64-DAG: shufflevector <32 x i8> %xGetHiInt8x32, <32 x i8> zeroinitializer, <16 x i32> <i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>
// LLVM-AMD64-DAG: define {{.*}} <32 x i8> @codegen.llvmShuffleGetHiInt8x64(<64 x i8> %xGetHiInt8x64
// LLVM-AMD64-DAG: shufflevector <64 x i8> %xGetHiInt8x64, <64 x i8> zeroinitializer, <32 x i32> <i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 48, i32 49, i32 50, i32 51, i32 52, i32 53, i32 54, i32 55, i32 56, i32 57, i32 58, i32 59, i32 60, i32 61, i32 62, i32 63>
// LLVM-AMD64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleGetHiUint16x16(<16 x i16> %xGetHiUint16x16
// LLVM-AMD64-DAG: shufflevector <16 x i16> %xGetHiUint16x16, <16 x i16> zeroinitializer, <8 x i32> <i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <16 x i16> @codegen.llvmShuffleGetHiUint16x32(<32 x i16> %xGetHiUint16x32
// LLVM-AMD64-DAG: shufflevector <32 x i16> %xGetHiUint16x32, <32 x i16> zeroinitializer, <16 x i32> <i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>
// LLVM-AMD64-DAG: define {{.*}} <8 x i32> @codegen.llvmShuffleGetHiUint32x16(<16 x i32> %xGetHiUint32x16
// LLVM-AMD64-DAG: shufflevector <16 x i32> %xGetHiUint32x16, <16 x i32> zeroinitializer, <8 x i32> <i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleGetHiUint32x8(<8 x i32> %xGetHiUint32x8
// LLVM-AMD64-DAG: shufflevector <8 x i32> %xGetHiUint32x8, <8 x i32> zeroinitializer, <4 x i32> <i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleGetHiUint64x4(<4 x i64> %xGetHiUint64x4
// LLVM-AMD64-DAG: shufflevector <4 x i64> %xGetHiUint64x4, <4 x i64> zeroinitializer, <2 x i32> <i32 2, i32 3>
// LLVM-AMD64-DAG: define {{.*}} <4 x i64> @codegen.llvmShuffleGetHiUint64x8(<8 x i64> %xGetHiUint64x8
// LLVM-AMD64-DAG: shufflevector <8 x i64> %xGetHiUint64x8, <8 x i64> zeroinitializer, <4 x i32> <i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <16 x i8> @codegen.llvmShuffleGetHiUint8x32(<32 x i8> %xGetHiUint8x32
// LLVM-AMD64-DAG: shufflevector <32 x i8> %xGetHiUint8x32, <32 x i8> zeroinitializer, <16 x i32> <i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>
// LLVM-AMD64-DAG: define {{.*}} <32 x i8> @codegen.llvmShuffleGetHiUint8x64(<64 x i8> %xGetHiUint8x64
// LLVM-AMD64-DAG: shufflevector <64 x i8> %xGetHiUint8x64, <64 x i8> zeroinitializer, <32 x i32> <i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 48, i32 49, i32 50, i32 51, i32 52, i32 53, i32 54, i32 55, i32 56, i32 57, i32 58, i32 59, i32 60, i32 61, i32 62, i32 63>
// LLVM-AMD64-DAG: define {{.*}} <8 x float> @codegen.llvmShuffleGetLoFloat32x16(<16 x float> %xGetLoFloat32x16
// LLVM-AMD64-DAG: shufflevector <16 x float> %xGetLoFloat32x16, <16 x float> zeroinitializer, <8 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <4 x float> @codegen.llvmShuffleGetLoFloat32x8(<8 x float> %xGetLoFloat32x8
// LLVM-AMD64-DAG: shufflevector <8 x float> %xGetLoFloat32x8, <8 x float> zeroinitializer, <4 x i32> <i32 0, i32 1, i32 2, i32 3>
// LLVM-AMD64-DAG: define {{.*}} <2 x double> @codegen.llvmShuffleGetLoFloat64x4(<4 x double> %xGetLoFloat64x4
// LLVM-AMD64-DAG: shufflevector <4 x double> %xGetLoFloat64x4, <4 x double> zeroinitializer, <2 x i32> <i32 0, i32 1>
// LLVM-AMD64-DAG: define {{.*}} <4 x double> @codegen.llvmShuffleGetLoFloat64x8(<8 x double> %xGetLoFloat64x8
// LLVM-AMD64-DAG: shufflevector <8 x double> %xGetLoFloat64x8, <8 x double> zeroinitializer, <4 x i32> <i32 0, i32 1, i32 2, i32 3>
// LLVM-AMD64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleGetLoInt16x16(<16 x i16> %xGetLoInt16x16
// LLVM-AMD64-DAG: shufflevector <16 x i16> %xGetLoInt16x16, <16 x i16> zeroinitializer, <8 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <16 x i16> @codegen.llvmShuffleGetLoInt16x32(<32 x i16> %xGetLoInt16x32
// LLVM-AMD64-DAG: shufflevector <32 x i16> %xGetLoInt16x32, <32 x i16> zeroinitializer, <16 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <8 x i32> @codegen.llvmShuffleGetLoInt32x16(<16 x i32> %xGetLoInt32x16
// LLVM-AMD64-DAG: shufflevector <16 x i32> %xGetLoInt32x16, <16 x i32> zeroinitializer, <8 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleGetLoInt32x8(<8 x i32> %xGetLoInt32x8
// LLVM-AMD64-DAG: shufflevector <8 x i32> %xGetLoInt32x8, <8 x i32> zeroinitializer, <4 x i32> <i32 0, i32 1, i32 2, i32 3>
// LLVM-AMD64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleGetLoInt64x4(<4 x i64> %xGetLoInt64x4
// LLVM-AMD64-DAG: shufflevector <4 x i64> %xGetLoInt64x4, <4 x i64> zeroinitializer, <2 x i32> <i32 0, i32 1>
// LLVM-AMD64-DAG: define {{.*}} <4 x i64> @codegen.llvmShuffleGetLoInt64x8(<8 x i64> %xGetLoInt64x8
// LLVM-AMD64-DAG: shufflevector <8 x i64> %xGetLoInt64x8, <8 x i64> zeroinitializer, <4 x i32> <i32 0, i32 1, i32 2, i32 3>
// LLVM-AMD64-DAG: define {{.*}} <16 x i8> @codegen.llvmShuffleGetLoInt8x32(<32 x i8> %xGetLoInt8x32
// LLVM-AMD64-DAG: shufflevector <32 x i8> %xGetLoInt8x32, <32 x i8> zeroinitializer, <16 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <32 x i8> @codegen.llvmShuffleGetLoInt8x64(<64 x i8> %xGetLoInt8x64
// LLVM-AMD64-DAG: shufflevector <64 x i8> %xGetLoInt8x64, <64 x i8> zeroinitializer, <32 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>
// LLVM-AMD64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleGetLoUint16x16(<16 x i16> %xGetLoUint16x16
// LLVM-AMD64-DAG: shufflevector <16 x i16> %xGetLoUint16x16, <16 x i16> zeroinitializer, <8 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <16 x i16> @codegen.llvmShuffleGetLoUint16x32(<32 x i16> %xGetLoUint16x32
// LLVM-AMD64-DAG: shufflevector <32 x i16> %xGetLoUint16x32, <32 x i16> zeroinitializer, <16 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <8 x i32> @codegen.llvmShuffleGetLoUint32x16(<16 x i32> %xGetLoUint32x16
// LLVM-AMD64-DAG: shufflevector <16 x i32> %xGetLoUint32x16, <16 x i32> zeroinitializer, <8 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleGetLoUint32x8(<8 x i32> %xGetLoUint32x8
// LLVM-AMD64-DAG: shufflevector <8 x i32> %xGetLoUint32x8, <8 x i32> zeroinitializer, <4 x i32> <i32 0, i32 1, i32 2, i32 3>
// LLVM-AMD64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleGetLoUint64x4(<4 x i64> %xGetLoUint64x4
// LLVM-AMD64-DAG: shufflevector <4 x i64> %xGetLoUint64x4, <4 x i64> zeroinitializer, <2 x i32> <i32 0, i32 1>
// LLVM-AMD64-DAG: define {{.*}} <4 x i64> @codegen.llvmShuffleGetLoUint64x8(<8 x i64> %xGetLoUint64x8
// LLVM-AMD64-DAG: shufflevector <8 x i64> %xGetLoUint64x8, <8 x i64> zeroinitializer, <4 x i32> <i32 0, i32 1, i32 2, i32 3>
// LLVM-AMD64-DAG: define {{.*}} <16 x i8> @codegen.llvmShuffleGetLoUint8x32(<32 x i8> %xGetLoUint8x32
// LLVM-AMD64-DAG: shufflevector <32 x i8> %xGetLoUint8x32, <32 x i8> zeroinitializer, <16 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <32 x i8> @codegen.llvmShuffleGetLoUint8x64(<64 x i8> %xGetLoUint8x64
// LLVM-AMD64-DAG: shufflevector <64 x i8> %xGetLoUint8x64, <64 x i8> zeroinitializer, <32 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>
// LLVM-AMD64-DAG: define {{.*}} <16 x i16> @codegen.llvmShuffleInterleaveHiGroupedInt16x16(<16 x i16> %xInterleaveHiGroupedInt16x16
// LLVM-AMD64-DAG: shufflevector <16 x i16> %xInterleaveHiGroupedInt16x16, <16 x i16> %yInterleaveHiGroupedInt16x16, <16 x i32> <i32 4, i32 20, i32 5, i32 21, i32 6, i32 22, i32 7, i32 23, i32 12, i32 28, i32 13, i32 29, i32 14, i32 30, i32 15, i32 31>
// LLVM-AMD64-DAG: define {{.*}} <32 x i16> @codegen.llvmShuffleInterleaveHiGroupedInt16x32(<32 x i16> %xInterleaveHiGroupedInt16x32
// LLVM-AMD64-DAG: shufflevector <32 x i16> %xInterleaveHiGroupedInt16x32, <32 x i16> %yInterleaveHiGroupedInt16x32, <32 x i32> <i32 4, i32 36, i32 5, i32 37, i32 6, i32 38, i32 7, i32 39, i32 12, i32 44, i32 13, i32 45, i32 14, i32 46, i32 15, i32 47, i32 20, i32 52, i32 21, i32 53, i32 22, i32 54, i32 23, i32 55, i32 28, i32 60, i32 29, i32 61, i32 30, i32 62, i32 31, i32 63>
// LLVM-AMD64-DAG: define {{.*}} <16 x i32> @codegen.llvmShuffleInterleaveHiGroupedInt32x16(<16 x i32> %xInterleaveHiGroupedInt32x16
// LLVM-AMD64-DAG: shufflevector <16 x i32> %xInterleaveHiGroupedInt32x16, <16 x i32> %yInterleaveHiGroupedInt32x16, <16 x i32> <i32 2, i32 18, i32 3, i32 19, i32 6, i32 22, i32 7, i32 23, i32 10, i32 26, i32 11, i32 27, i32 14, i32 30, i32 15, i32 31>
// LLVM-AMD64-DAG: define {{.*}} <8 x i32> @codegen.llvmShuffleInterleaveHiGroupedInt32x8(<8 x i32> %xInterleaveHiGroupedInt32x8
// LLVM-AMD64-DAG: shufflevector <8 x i32> %xInterleaveHiGroupedInt32x8, <8 x i32> %yInterleaveHiGroupedInt32x8, <8 x i32> <i32 2, i32 10, i32 3, i32 11, i32 6, i32 14, i32 7, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <4 x i64> @codegen.llvmShuffleInterleaveHiGroupedInt64x4(<4 x i64> %xInterleaveHiGroupedInt64x4
// LLVM-AMD64-DAG: shufflevector <4 x i64> %xInterleaveHiGroupedInt64x4, <4 x i64> %yInterleaveHiGroupedInt64x4, <4 x i32> <i32 1, i32 5, i32 3, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <8 x i64> @codegen.llvmShuffleInterleaveHiGroupedInt64x8(<8 x i64> %xInterleaveHiGroupedInt64x8
// LLVM-AMD64-DAG: shufflevector <8 x i64> %xInterleaveHiGroupedInt64x8, <8 x i64> %yInterleaveHiGroupedInt64x8, <8 x i32> <i32 1, i32 9, i32 3, i32 11, i32 5, i32 13, i32 7, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <16 x i16> @codegen.llvmShuffleInterleaveHiGroupedUint16x16(<16 x i16> %xInterleaveHiGroupedUint16x16
// LLVM-AMD64-DAG: shufflevector <16 x i16> %xInterleaveHiGroupedUint16x16, <16 x i16> %yInterleaveHiGroupedUint16x16, <16 x i32> <i32 4, i32 20, i32 5, i32 21, i32 6, i32 22, i32 7, i32 23, i32 12, i32 28, i32 13, i32 29, i32 14, i32 30, i32 15, i32 31>
// LLVM-AMD64-DAG: define {{.*}} <32 x i16> @codegen.llvmShuffleInterleaveHiGroupedUint16x32(<32 x i16> %xInterleaveHiGroupedUint16x32
// LLVM-AMD64-DAG: shufflevector <32 x i16> %xInterleaveHiGroupedUint16x32, <32 x i16> %yInterleaveHiGroupedUint16x32, <32 x i32> <i32 4, i32 36, i32 5, i32 37, i32 6, i32 38, i32 7, i32 39, i32 12, i32 44, i32 13, i32 45, i32 14, i32 46, i32 15, i32 47, i32 20, i32 52, i32 21, i32 53, i32 22, i32 54, i32 23, i32 55, i32 28, i32 60, i32 29, i32 61, i32 30, i32 62, i32 31, i32 63>
// LLVM-AMD64-DAG: define {{.*}} <16 x i32> @codegen.llvmShuffleInterleaveHiGroupedUint32x16(<16 x i32> %xInterleaveHiGroupedUint32x16
// LLVM-AMD64-DAG: shufflevector <16 x i32> %xInterleaveHiGroupedUint32x16, <16 x i32> %yInterleaveHiGroupedUint32x16, <16 x i32> <i32 2, i32 18, i32 3, i32 19, i32 6, i32 22, i32 7, i32 23, i32 10, i32 26, i32 11, i32 27, i32 14, i32 30, i32 15, i32 31>
// LLVM-AMD64-DAG: define {{.*}} <8 x i32> @codegen.llvmShuffleInterleaveHiGroupedUint32x8(<8 x i32> %xInterleaveHiGroupedUint32x8
// LLVM-AMD64-DAG: shufflevector <8 x i32> %xInterleaveHiGroupedUint32x8, <8 x i32> %yInterleaveHiGroupedUint32x8, <8 x i32> <i32 2, i32 10, i32 3, i32 11, i32 6, i32 14, i32 7, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <4 x i64> @codegen.llvmShuffleInterleaveHiGroupedUint64x4(<4 x i64> %xInterleaveHiGroupedUint64x4
// LLVM-AMD64-DAG: shufflevector <4 x i64> %xInterleaveHiGroupedUint64x4, <4 x i64> %yInterleaveHiGroupedUint64x4, <4 x i32> <i32 1, i32 5, i32 3, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <8 x i64> @codegen.llvmShuffleInterleaveHiGroupedUint64x8(<8 x i64> %xInterleaveHiGroupedUint64x8
// LLVM-AMD64-DAG: shufflevector <8 x i64> %xInterleaveHiGroupedUint64x8, <8 x i64> %yInterleaveHiGroupedUint64x8, <8 x i32> <i32 1, i32 9, i32 3, i32 11, i32 5, i32 13, i32 7, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleInterleaveHiInt16x8(<8 x i16> %xInterleaveHiInt16x8
// LLVM-AMD64-DAG: shufflevector <8 x i16> %xInterleaveHiInt16x8, <8 x i16> %yInterleaveHiInt16x8, <8 x i32> <i32 4, i32 12, i32 5, i32 13, i32 6, i32 14, i32 7, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleInterleaveHiInt32x4(<4 x i32> %xInterleaveHiInt32x4
// LLVM-AMD64-DAG: shufflevector <4 x i32> %xInterleaveHiInt32x4, <4 x i32> %yInterleaveHiInt32x4, <4 x i32> <i32 2, i32 6, i32 3, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleInterleaveHiInt64x2(<2 x i64> %xInterleaveHiInt64x2
// LLVM-AMD64-DAG: shufflevector <2 x i64> %xInterleaveHiInt64x2, <2 x i64> %yInterleaveHiInt64x2, <2 x i32> <i32 1, i32 3>
// LLVM-AMD64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleInterleaveHiUint16x8(<8 x i16> %xInterleaveHiUint16x8
// LLVM-AMD64-DAG: shufflevector <8 x i16> %xInterleaveHiUint16x8, <8 x i16> %yInterleaveHiUint16x8, <8 x i32> <i32 4, i32 12, i32 5, i32 13, i32 6, i32 14, i32 7, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleInterleaveHiUint32x4(<4 x i32> %xInterleaveHiUint32x4
// LLVM-AMD64-DAG: shufflevector <4 x i32> %xInterleaveHiUint32x4, <4 x i32> %yInterleaveHiUint32x4, <4 x i32> <i32 2, i32 6, i32 3, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleInterleaveHiUint64x2(<2 x i64> %xInterleaveHiUint64x2
// LLVM-AMD64-DAG: shufflevector <2 x i64> %xInterleaveHiUint64x2, <2 x i64> %yInterleaveHiUint64x2, <2 x i32> <i32 1, i32 3>
// LLVM-AMD64-DAG: define {{.*}} <16 x i16> @codegen.llvmShuffleInterleaveLoGroupedInt16x16(<16 x i16> %xInterleaveLoGroupedInt16x16
// LLVM-AMD64-DAG: shufflevector <16 x i16> %xInterleaveLoGroupedInt16x16, <16 x i16> %yInterleaveLoGroupedInt16x16, <16 x i32> <i32 0, i32 16, i32 1, i32 17, i32 2, i32 18, i32 3, i32 19, i32 8, i32 24, i32 9, i32 25, i32 10, i32 26, i32 11, i32 27>
// LLVM-AMD64-DAG: define {{.*}} <32 x i16> @codegen.llvmShuffleInterleaveLoGroupedInt16x32(<32 x i16> %xInterleaveLoGroupedInt16x32
// LLVM-AMD64-DAG: shufflevector <32 x i16> %xInterleaveLoGroupedInt16x32, <32 x i16> %yInterleaveLoGroupedInt16x32, <32 x i32> <i32 0, i32 32, i32 1, i32 33, i32 2, i32 34, i32 3, i32 35, i32 8, i32 40, i32 9, i32 41, i32 10, i32 42, i32 11, i32 43, i32 16, i32 48, i32 17, i32 49, i32 18, i32 50, i32 19, i32 51, i32 24, i32 56, i32 25, i32 57, i32 26, i32 58, i32 27, i32 59>
// LLVM-AMD64-DAG: define {{.*}} <16 x i32> @codegen.llvmShuffleInterleaveLoGroupedInt32x16(<16 x i32> %xInterleaveLoGroupedInt32x16
// LLVM-AMD64-DAG: shufflevector <16 x i32> %xInterleaveLoGroupedInt32x16, <16 x i32> %yInterleaveLoGroupedInt32x16, <16 x i32> <i32 0, i32 16, i32 1, i32 17, i32 4, i32 20, i32 5, i32 21, i32 8, i32 24, i32 9, i32 25, i32 12, i32 28, i32 13, i32 29>
// LLVM-AMD64-DAG: define {{.*}} <8 x i32> @codegen.llvmShuffleInterleaveLoGroupedInt32x8(<8 x i32> %xInterleaveLoGroupedInt32x8
// LLVM-AMD64-DAG: shufflevector <8 x i32> %xInterleaveLoGroupedInt32x8, <8 x i32> %yInterleaveLoGroupedInt32x8, <8 x i32> <i32 0, i32 8, i32 1, i32 9, i32 4, i32 12, i32 5, i32 13>
// LLVM-AMD64-DAG: define {{.*}} <4 x i64> @codegen.llvmShuffleInterleaveLoGroupedInt64x4(<4 x i64> %xInterleaveLoGroupedInt64x4
// LLVM-AMD64-DAG: shufflevector <4 x i64> %xInterleaveLoGroupedInt64x4, <4 x i64> %yInterleaveLoGroupedInt64x4, <4 x i32> <i32 0, i32 4, i32 2, i32 6>
// LLVM-AMD64-DAG: define {{.*}} <8 x i64> @codegen.llvmShuffleInterleaveLoGroupedInt64x8(<8 x i64> %xInterleaveLoGroupedInt64x8
// LLVM-AMD64-DAG: shufflevector <8 x i64> %xInterleaveLoGroupedInt64x8, <8 x i64> %yInterleaveLoGroupedInt64x8, <8 x i32> <i32 0, i32 8, i32 2, i32 10, i32 4, i32 12, i32 6, i32 14>
// LLVM-AMD64-DAG: define {{.*}} <16 x i16> @codegen.llvmShuffleInterleaveLoGroupedUint16x16(<16 x i16> %xInterleaveLoGroupedUint16x16
// LLVM-AMD64-DAG: shufflevector <16 x i16> %xInterleaveLoGroupedUint16x16, <16 x i16> %yInterleaveLoGroupedUint16x16, <16 x i32> <i32 0, i32 16, i32 1, i32 17, i32 2, i32 18, i32 3, i32 19, i32 8, i32 24, i32 9, i32 25, i32 10, i32 26, i32 11, i32 27>
// LLVM-AMD64-DAG: define {{.*}} <32 x i16> @codegen.llvmShuffleInterleaveLoGroupedUint16x32(<32 x i16> %xInterleaveLoGroupedUint16x32
// LLVM-AMD64-DAG: shufflevector <32 x i16> %xInterleaveLoGroupedUint16x32, <32 x i16> %yInterleaveLoGroupedUint16x32, <32 x i32> <i32 0, i32 32, i32 1, i32 33, i32 2, i32 34, i32 3, i32 35, i32 8, i32 40, i32 9, i32 41, i32 10, i32 42, i32 11, i32 43, i32 16, i32 48, i32 17, i32 49, i32 18, i32 50, i32 19, i32 51, i32 24, i32 56, i32 25, i32 57, i32 26, i32 58, i32 27, i32 59>
// LLVM-AMD64-DAG: define {{.*}} <16 x i32> @codegen.llvmShuffleInterleaveLoGroupedUint32x16(<16 x i32> %xInterleaveLoGroupedUint32x16
// LLVM-AMD64-DAG: shufflevector <16 x i32> %xInterleaveLoGroupedUint32x16, <16 x i32> %yInterleaveLoGroupedUint32x16, <16 x i32> <i32 0, i32 16, i32 1, i32 17, i32 4, i32 20, i32 5, i32 21, i32 8, i32 24, i32 9, i32 25, i32 12, i32 28, i32 13, i32 29>
// LLVM-AMD64-DAG: define {{.*}} <8 x i32> @codegen.llvmShuffleInterleaveLoGroupedUint32x8(<8 x i32> %xInterleaveLoGroupedUint32x8
// LLVM-AMD64-DAG: shufflevector <8 x i32> %xInterleaveLoGroupedUint32x8, <8 x i32> %yInterleaveLoGroupedUint32x8, <8 x i32> <i32 0, i32 8, i32 1, i32 9, i32 4, i32 12, i32 5, i32 13>
// LLVM-AMD64-DAG: define {{.*}} <4 x i64> @codegen.llvmShuffleInterleaveLoGroupedUint64x4(<4 x i64> %xInterleaveLoGroupedUint64x4
// LLVM-AMD64-DAG: shufflevector <4 x i64> %xInterleaveLoGroupedUint64x4, <4 x i64> %yInterleaveLoGroupedUint64x4, <4 x i32> <i32 0, i32 4, i32 2, i32 6>
// LLVM-AMD64-DAG: define {{.*}} <8 x i64> @codegen.llvmShuffleInterleaveLoGroupedUint64x8(<8 x i64> %xInterleaveLoGroupedUint64x8
// LLVM-AMD64-DAG: shufflevector <8 x i64> %xInterleaveLoGroupedUint64x8, <8 x i64> %yInterleaveLoGroupedUint64x8, <8 x i32> <i32 0, i32 8, i32 2, i32 10, i32 4, i32 12, i32 6, i32 14>
// LLVM-AMD64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleInterleaveLoInt16x8(<8 x i16> %xInterleaveLoInt16x8
// LLVM-AMD64-DAG: shufflevector <8 x i16> %xInterleaveLoInt16x8, <8 x i16> %yInterleaveLoInt16x8, <8 x i32> <i32 0, i32 8, i32 1, i32 9, i32 2, i32 10, i32 3, i32 11>
// LLVM-AMD64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleInterleaveLoInt32x4(<4 x i32> %xInterleaveLoInt32x4
// LLVM-AMD64-DAG: shufflevector <4 x i32> %xInterleaveLoInt32x4, <4 x i32> %yInterleaveLoInt32x4, <4 x i32> <i32 0, i32 4, i32 1, i32 5>
// LLVM-AMD64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleInterleaveLoInt64x2(<2 x i64> %xInterleaveLoInt64x2
// LLVM-AMD64-DAG: shufflevector <2 x i64> %xInterleaveLoInt64x2, <2 x i64> %yInterleaveLoInt64x2, <2 x i32> <i32 0, i32 2>
// LLVM-AMD64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleInterleaveLoUint16x8(<8 x i16> %xInterleaveLoUint16x8
// LLVM-AMD64-DAG: shufflevector <8 x i16> %xInterleaveLoUint16x8, <8 x i16> %yInterleaveLoUint16x8, <8 x i32> <i32 0, i32 8, i32 1, i32 9, i32 2, i32 10, i32 3, i32 11>
// LLVM-AMD64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleInterleaveLoUint32x4(<4 x i32> %xInterleaveLoUint32x4
// LLVM-AMD64-DAG: shufflevector <4 x i32> %xInterleaveLoUint32x4, <4 x i32> %yInterleaveLoUint32x4, <4 x i32> <i32 0, i32 4, i32 1, i32 5>
// LLVM-AMD64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleInterleaveLoUint64x2(<2 x i64> %xInterleaveLoUint64x2
// LLVM-AMD64-DAG: shufflevector <2 x i64> %xInterleaveLoUint64x2, <2 x i64> %yInterleaveLoUint64x2, <2 x i32> <i32 0, i32 2>
// LLVM-AMD64-DAG: define {{.*}} <16 x float> @codegen.llvmShuffleSetHiFloat32x16(<16 x float> %xSetHiFloat32x16
// LLVM-AMD64-DAG: shufflevector <16 x float> %xSetHiFloat32x16, <16 x float> {{%[^,]+}}, <16 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23>
// LLVM-AMD64-DAG: define {{.*}} <8 x float> @codegen.llvmShuffleSetHiFloat32x8(<8 x float> %xSetHiFloat32x8
// LLVM-AMD64-DAG: shufflevector <8 x float> %xSetHiFloat32x8, <8 x float> {{%[^,]+}}, <8 x i32> <i32 0, i32 1, i32 2, i32 3, i32 8, i32 9, i32 10, i32 11>
// LLVM-AMD64-DAG: define {{.*}} <4 x double> @codegen.llvmShuffleSetHiFloat64x4(<4 x double> %xSetHiFloat64x4
// LLVM-AMD64-DAG: shufflevector <4 x double> %xSetHiFloat64x4, <4 x double> {{%[^,]+}}, <4 x i32> <i32 0, i32 1, i32 4, i32 5>
// LLVM-AMD64-DAG: define {{.*}} <8 x double> @codegen.llvmShuffleSetHiFloat64x8(<8 x double> %xSetHiFloat64x8
// LLVM-AMD64-DAG: shufflevector <8 x double> %xSetHiFloat64x8, <8 x double> {{%[^,]+}}, <8 x i32> <i32 0, i32 1, i32 2, i32 3, i32 8, i32 9, i32 10, i32 11>
// LLVM-AMD64-DAG: define {{.*}} <16 x i16> @codegen.llvmShuffleSetHiInt16x16(<16 x i16> %xSetHiInt16x16
// LLVM-AMD64-DAG: shufflevector <16 x i16> %xSetHiInt16x16, <16 x i16> {{%[^,]+}}, <16 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23>
// LLVM-AMD64-DAG: define {{.*}} <32 x i16> @codegen.llvmShuffleSetHiInt16x32(<32 x i16> %xSetHiInt16x32
// LLVM-AMD64-DAG: shufflevector <32 x i16> %xSetHiInt16x32, <32 x i16> {{%[^,]+}}, <32 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15, i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47>
// LLVM-AMD64-DAG: define {{.*}} <16 x i32> @codegen.llvmShuffleSetHiInt32x16(<16 x i32> %xSetHiInt32x16
// LLVM-AMD64-DAG: shufflevector <16 x i32> %xSetHiInt32x16, <16 x i32> {{%[^,]+}}, <16 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23>
// LLVM-AMD64-DAG: define {{.*}} <8 x i32> @codegen.llvmShuffleSetHiInt32x8(<8 x i32> %xSetHiInt32x8
// LLVM-AMD64-DAG: shufflevector <8 x i32> %xSetHiInt32x8, <8 x i32> {{%[^,]+}}, <8 x i32> <i32 0, i32 1, i32 2, i32 3, i32 8, i32 9, i32 10, i32 11>
// LLVM-AMD64-DAG: define {{.*}} <4 x i64> @codegen.llvmShuffleSetHiInt64x4(<4 x i64> %xSetHiInt64x4
// LLVM-AMD64-DAG: shufflevector <4 x i64> %xSetHiInt64x4, <4 x i64> {{%[^,]+}}, <4 x i32> <i32 0, i32 1, i32 4, i32 5>
// LLVM-AMD64-DAG: define {{.*}} <8 x i64> @codegen.llvmShuffleSetHiInt64x8(<8 x i64> %xSetHiInt64x8
// LLVM-AMD64-DAG: shufflevector <8 x i64> %xSetHiInt64x8, <8 x i64> {{%[^,]+}}, <8 x i32> <i32 0, i32 1, i32 2, i32 3, i32 8, i32 9, i32 10, i32 11>
// LLVM-AMD64-DAG: define {{.*}} <32 x i8> @codegen.llvmShuffleSetHiInt8x32(<32 x i8> %xSetHiInt8x32
// LLVM-AMD64-DAG: shufflevector <32 x i8> %xSetHiInt8x32, <32 x i8> {{%[^,]+}}, <32 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15, i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47>
// LLVM-AMD64-DAG: define {{.*}} <64 x i8> @codegen.llvmShuffleSetHiInt8x64(<64 x i8> %xSetHiInt8x64
// LLVM-AMD64-DAG: shufflevector <64 x i8> %xSetHiInt8x64, <64 x i8> {{%[^,]+}}, <64 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31, i32 64, i32 65, i32 66, i32 67, i32 68, i32 69, i32 70, i32 71, i32 72, i32 73, i32 74, i32 75, i32 76, i32 77, i32 78, i32 79, i32 80, i32 81, i32 82, i32 83, i32 84, i32 85, i32 86, i32 87, i32 88, i32 89, i32 90, i32 91, i32 92, i32 93, i32 94, i32 95>
// LLVM-AMD64-DAG: define {{.*}} <16 x i16> @codegen.llvmShuffleSetHiUint16x16(<16 x i16> %xSetHiUint16x16
// LLVM-AMD64-DAG: shufflevector <16 x i16> %xSetHiUint16x16, <16 x i16> {{%[^,]+}}, <16 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23>
// LLVM-AMD64-DAG: define {{.*}} <32 x i16> @codegen.llvmShuffleSetHiUint16x32(<32 x i16> %xSetHiUint16x32
// LLVM-AMD64-DAG: shufflevector <32 x i16> %xSetHiUint16x32, <32 x i16> {{%[^,]+}}, <32 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15, i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47>
// LLVM-AMD64-DAG: define {{.*}} <16 x i32> @codegen.llvmShuffleSetHiUint32x16(<16 x i32> %xSetHiUint32x16
// LLVM-AMD64-DAG: shufflevector <16 x i32> %xSetHiUint32x16, <16 x i32> {{%[^,]+}}, <16 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23>
// LLVM-AMD64-DAG: define {{.*}} <8 x i32> @codegen.llvmShuffleSetHiUint32x8(<8 x i32> %xSetHiUint32x8
// LLVM-AMD64-DAG: shufflevector <8 x i32> %xSetHiUint32x8, <8 x i32> {{%[^,]+}}, <8 x i32> <i32 0, i32 1, i32 2, i32 3, i32 8, i32 9, i32 10, i32 11>
// LLVM-AMD64-DAG: define {{.*}} <4 x i64> @codegen.llvmShuffleSetHiUint64x4(<4 x i64> %xSetHiUint64x4
// LLVM-AMD64-DAG: shufflevector <4 x i64> %xSetHiUint64x4, <4 x i64> {{%[^,]+}}, <4 x i32> <i32 0, i32 1, i32 4, i32 5>
// LLVM-AMD64-DAG: define {{.*}} <8 x i64> @codegen.llvmShuffleSetHiUint64x8(<8 x i64> %xSetHiUint64x8
// LLVM-AMD64-DAG: shufflevector <8 x i64> %xSetHiUint64x8, <8 x i64> {{%[^,]+}}, <8 x i32> <i32 0, i32 1, i32 2, i32 3, i32 8, i32 9, i32 10, i32 11>
// LLVM-AMD64-DAG: define {{.*}} <32 x i8> @codegen.llvmShuffleSetHiUint8x32(<32 x i8> %xSetHiUint8x32
// LLVM-AMD64-DAG: shufflevector <32 x i8> %xSetHiUint8x32, <32 x i8> {{%[^,]+}}, <32 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15, i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47>
// LLVM-AMD64-DAG: define {{.*}} <64 x i8> @codegen.llvmShuffleSetHiUint8x64(<64 x i8> %xSetHiUint8x64
// LLVM-AMD64-DAG: shufflevector <64 x i8> %xSetHiUint8x64, <64 x i8> {{%[^,]+}}, <64 x i32> <i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31, i32 64, i32 65, i32 66, i32 67, i32 68, i32 69, i32 70, i32 71, i32 72, i32 73, i32 74, i32 75, i32 76, i32 77, i32 78, i32 79, i32 80, i32 81, i32 82, i32 83, i32 84, i32 85, i32 86, i32 87, i32 88, i32 89, i32 90, i32 91, i32 92, i32 93, i32 94, i32 95>
// LLVM-AMD64-DAG: define {{.*}} <16 x float> @codegen.llvmShuffleSetLoFloat32x16(<16 x float> %xSetLoFloat32x16
// LLVM-AMD64-DAG: shufflevector <16 x float> %xSetLoFloat32x16, <16 x float> {{%[^,]+}}, <16 x i32> <i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <8 x float> @codegen.llvmShuffleSetLoFloat32x8(<8 x float> %xSetLoFloat32x8
// LLVM-AMD64-DAG: shufflevector <8 x float> %xSetLoFloat32x8, <8 x float> {{%[^,]+}}, <8 x i32> <i32 8, i32 9, i32 10, i32 11, i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <4 x double> @codegen.llvmShuffleSetLoFloat64x4(<4 x double> %xSetLoFloat64x4
// LLVM-AMD64-DAG: shufflevector <4 x double> %xSetLoFloat64x4, <4 x double> {{%[^,]+}}, <4 x i32> <i32 4, i32 5, i32 2, i32 3>
// LLVM-AMD64-DAG: define {{.*}} <8 x double> @codegen.llvmShuffleSetLoFloat64x8(<8 x double> %xSetLoFloat64x8
// LLVM-AMD64-DAG: shufflevector <8 x double> %xSetLoFloat64x8, <8 x double> {{%[^,]+}}, <8 x i32> <i32 8, i32 9, i32 10, i32 11, i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <16 x i16> @codegen.llvmShuffleSetLoInt16x16(<16 x i16> %xSetLoInt16x16
// LLVM-AMD64-DAG: shufflevector <16 x i16> %xSetLoInt16x16, <16 x i16> {{%[^,]+}}, <16 x i32> <i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <32 x i16> @codegen.llvmShuffleSetLoInt16x32(<32 x i16> %xSetLoInt16x32
// LLVM-AMD64-DAG: shufflevector <32 x i16> %xSetLoInt16x32, <32 x i16> {{%[^,]+}}, <32 x i32> <i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>
// LLVM-AMD64-DAG: define {{.*}} <16 x i32> @codegen.llvmShuffleSetLoInt32x16(<16 x i32> %xSetLoInt32x16
// LLVM-AMD64-DAG: shufflevector <16 x i32> %xSetLoInt32x16, <16 x i32> {{%[^,]+}}, <16 x i32> <i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <8 x i32> @codegen.llvmShuffleSetLoInt32x8(<8 x i32> %xSetLoInt32x8
// LLVM-AMD64-DAG: shufflevector <8 x i32> %xSetLoInt32x8, <8 x i32> {{%[^,]+}}, <8 x i32> <i32 8, i32 9, i32 10, i32 11, i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <4 x i64> @codegen.llvmShuffleSetLoInt64x4(<4 x i64> %xSetLoInt64x4
// LLVM-AMD64-DAG: shufflevector <4 x i64> %xSetLoInt64x4, <4 x i64> {{%[^,]+}}, <4 x i32> <i32 4, i32 5, i32 2, i32 3>
// LLVM-AMD64-DAG: define {{.*}} <8 x i64> @codegen.llvmShuffleSetLoInt64x8(<8 x i64> %xSetLoInt64x8
// LLVM-AMD64-DAG: shufflevector <8 x i64> %xSetLoInt64x8, <8 x i64> {{%[^,]+}}, <8 x i32> <i32 8, i32 9, i32 10, i32 11, i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <32 x i8> @codegen.llvmShuffleSetLoInt8x32(<32 x i8> %xSetLoInt8x32
// LLVM-AMD64-DAG: shufflevector <32 x i8> %xSetLoInt8x32, <32 x i8> {{%[^,]+}}, <32 x i32> <i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>
// LLVM-AMD64-DAG: define {{.*}} <64 x i8> @codegen.llvmShuffleSetLoInt8x64(<64 x i8> %xSetLoInt8x64
// LLVM-AMD64-DAG: shufflevector <64 x i8> %xSetLoInt8x64, <64 x i8> {{%[^,]+}}, <64 x i32> <i32 64, i32 65, i32 66, i32 67, i32 68, i32 69, i32 70, i32 71, i32 72, i32 73, i32 74, i32 75, i32 76, i32 77, i32 78, i32 79, i32 80, i32 81, i32 82, i32 83, i32 84, i32 85, i32 86, i32 87, i32 88, i32 89, i32 90, i32 91, i32 92, i32 93, i32 94, i32 95, i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 48, i32 49, i32 50, i32 51, i32 52, i32 53, i32 54, i32 55, i32 56, i32 57, i32 58, i32 59, i32 60, i32 61, i32 62, i32 63>
// LLVM-AMD64-DAG: define {{.*}} <16 x i16> @codegen.llvmShuffleSetLoUint16x16(<16 x i16> %xSetLoUint16x16
// LLVM-AMD64-DAG: shufflevector <16 x i16> %xSetLoUint16x16, <16 x i16> {{%[^,]+}}, <16 x i32> <i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <32 x i16> @codegen.llvmShuffleSetLoUint16x32(<32 x i16> %xSetLoUint16x32
// LLVM-AMD64-DAG: shufflevector <32 x i16> %xSetLoUint16x32, <32 x i16> {{%[^,]+}}, <32 x i32> <i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>
// LLVM-AMD64-DAG: define {{.*}} <16 x i32> @codegen.llvmShuffleSetLoUint32x16(<16 x i32> %xSetLoUint32x16
// LLVM-AMD64-DAG: shufflevector <16 x i32> %xSetLoUint32x16, <16 x i32> {{%[^,]+}}, <16 x i32> <i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>
// LLVM-AMD64-DAG: define {{.*}} <8 x i32> @codegen.llvmShuffleSetLoUint32x8(<8 x i32> %xSetLoUint32x8
// LLVM-AMD64-DAG: shufflevector <8 x i32> %xSetLoUint32x8, <8 x i32> {{%[^,]+}}, <8 x i32> <i32 8, i32 9, i32 10, i32 11, i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <4 x i64> @codegen.llvmShuffleSetLoUint64x4(<4 x i64> %xSetLoUint64x4
// LLVM-AMD64-DAG: shufflevector <4 x i64> %xSetLoUint64x4, <4 x i64> {{%[^,]+}}, <4 x i32> <i32 4, i32 5, i32 2, i32 3>
// LLVM-AMD64-DAG: define {{.*}} <8 x i64> @codegen.llvmShuffleSetLoUint64x8(<8 x i64> %xSetLoUint64x8
// LLVM-AMD64-DAG: shufflevector <8 x i64> %xSetLoUint64x8, <8 x i64> {{%[^,]+}}, <8 x i32> <i32 8, i32 9, i32 10, i32 11, i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: define {{.*}} <32 x i8> @codegen.llvmShuffleSetLoUint8x32(<32 x i8> %xSetLoUint8x32
// LLVM-AMD64-DAG: shufflevector <32 x i8> %xSetLoUint8x32, <32 x i8> {{%[^,]+}}, <32 x i32> <i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>
// LLVM-AMD64-DAG: define {{.*}} <64 x i8> @codegen.llvmShuffleSetLoUint8x64(<64 x i8> %xSetLoUint8x64
// LLVM-AMD64-DAG: shufflevector <64 x i8> %xSetLoUint8x64, <64 x i8> {{%[^,]+}}, <64 x i32> <i32 64, i32 65, i32 66, i32 67, i32 68, i32 69, i32 70, i32 71, i32 72, i32 73, i32 74, i32 75, i32 76, i32 77, i32 78, i32 79, i32 80, i32 81, i32 82, i32 83, i32 84, i32 85, i32 86, i32 87, i32 88, i32 89, i32 90, i32 91, i32 92, i32 93, i32 94, i32 95, i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 48, i32 49, i32 50, i32 51, i32 52, i32 53, i32 54, i32 55, i32 56, i32 57, i32 58, i32 59, i32 60, i32 61, i32 62, i32 63>

//go:noinline
func llvmShufflebroadcast1To16Float32x4(xbroadcast1To16Float32x4 archsimd.Float32x4) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return archsimd.BroadcastFloat32x16(xbroadcast1To16Float32x4.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To16Int16x8(xbroadcast1To16Int16x8 archsimd.Int16x8) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return archsimd.BroadcastInt16x16(xbroadcast1To16Int16x8.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To16Int32x4(xbroadcast1To16Int32x4 archsimd.Int32x4) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return archsimd.BroadcastInt32x16(xbroadcast1To16Int32x4.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To16Int8x16(xbroadcast1To16Int8x16 archsimd.Int8x16) archsimd.Int8x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int8x16{}
	}
	return archsimd.BroadcastInt8x16(xbroadcast1To16Int8x16.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To16Uint16x8(xbroadcast1To16Uint16x8 archsimd.Uint16x8) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return archsimd.BroadcastUint16x16(xbroadcast1To16Uint16x8.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To16Uint32x4(xbroadcast1To16Uint32x4 archsimd.Uint32x4) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return archsimd.BroadcastUint32x16(xbroadcast1To16Uint32x4.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To16Uint8x16(xbroadcast1To16Uint8x16 archsimd.Uint8x16) archsimd.Uint8x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x16{}
	}
	return archsimd.BroadcastUint8x16(xbroadcast1To16Uint8x16.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To2Float64x2(xbroadcast1To2Float64x2 archsimd.Float64x2) archsimd.Float64x2 {
	if !archsimd.X86.AVX2() {
		return archsimd.Float64x2{}
	}
	return archsimd.BroadcastFloat64x2(xbroadcast1To2Float64x2.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To2Int64x2(xbroadcast1To2Int64x2 archsimd.Int64x2) archsimd.Int64x2 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x2{}
	}
	return archsimd.BroadcastInt64x2(xbroadcast1To2Int64x2.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To2Uint64x2(xbroadcast1To2Uint64x2 archsimd.Uint64x2) archsimd.Uint64x2 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x2{}
	}
	return archsimd.BroadcastUint64x2(xbroadcast1To2Uint64x2.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To32Int16x8(xbroadcast1To32Int16x8 archsimd.Int16x8) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return archsimd.BroadcastInt16x32(xbroadcast1To32Int16x8.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To32Int8x16(xbroadcast1To32Int8x16 archsimd.Int8x16) archsimd.Int8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int8x32{}
	}
	return archsimd.BroadcastInt8x32(xbroadcast1To32Int8x16.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To32Uint16x8(xbroadcast1To32Uint16x8 archsimd.Uint16x8) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return archsimd.BroadcastUint16x32(xbroadcast1To32Uint16x8.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To32Uint8x16(xbroadcast1To32Uint8x16 archsimd.Uint8x16) archsimd.Uint8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x32{}
	}
	return archsimd.BroadcastUint8x32(xbroadcast1To32Uint8x16.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To4Float32x4(xbroadcast1To4Float32x4 archsimd.Float32x4) archsimd.Float32x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Float32x4{}
	}
	return archsimd.BroadcastFloat32x4(xbroadcast1To4Float32x4.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To4Float64x2(xbroadcast1To4Float64x2 archsimd.Float64x2) archsimd.Float64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Float64x4{}
	}
	return archsimd.BroadcastFloat64x4(xbroadcast1To4Float64x2.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To4Int32x4(xbroadcast1To4Int32x4 archsimd.Int32x4) archsimd.Int32x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x4{}
	}
	return archsimd.BroadcastInt32x4(xbroadcast1To4Int32x4.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To4Int64x2(xbroadcast1To4Int64x2 archsimd.Int64x2) archsimd.Int64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x4{}
	}
	return archsimd.BroadcastInt64x4(xbroadcast1To4Int64x2.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To4Uint32x4(xbroadcast1To4Uint32x4 archsimd.Uint32x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x4{}
	}
	return archsimd.BroadcastUint32x4(xbroadcast1To4Uint32x4.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To4Uint64x2(xbroadcast1To4Uint64x2 archsimd.Uint64x2) archsimd.Uint64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x4{}
	}
	return archsimd.BroadcastUint64x4(xbroadcast1To4Uint64x2.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To64Int8x16(xbroadcast1To64Int8x16 archsimd.Int8x16) archsimd.Int8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x64{}
	}
	return archsimd.BroadcastInt8x64(xbroadcast1To64Int8x16.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To64Uint8x16(xbroadcast1To64Uint8x16 archsimd.Uint8x16) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x64{}
	}
	return archsimd.BroadcastUint8x64(xbroadcast1To64Uint8x16.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To8Float32x4(xbroadcast1To8Float32x4 archsimd.Float32x4) archsimd.Float32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Float32x8{}
	}
	return archsimd.BroadcastFloat32x8(xbroadcast1To8Float32x4.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To8Float64x2(xbroadcast1To8Float64x2 archsimd.Float64x2) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return archsimd.BroadcastFloat64x8(xbroadcast1To8Float64x2.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To8Int16x8(xbroadcast1To8Int16x8 archsimd.Int16x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x8{}
	}
	return archsimd.BroadcastInt16x8(xbroadcast1To8Int16x8.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To8Int32x4(xbroadcast1To8Int32x4 archsimd.Int32x4) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return archsimd.BroadcastInt32x8(xbroadcast1To8Int32x4.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To8Int64x2(xbroadcast1To8Int64x2 archsimd.Int64x2) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return archsimd.BroadcastInt64x8(xbroadcast1To8Int64x2.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To8Uint16x8(xbroadcast1To8Uint16x8 archsimd.Uint16x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x8{}
	}
	return archsimd.BroadcastUint16x8(xbroadcast1To8Uint16x8.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To8Uint32x4(xbroadcast1To8Uint32x4 archsimd.Uint32x4) archsimd.Uint32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x8{}
	}
	return archsimd.BroadcastUint32x8(xbroadcast1To8Uint32x4.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To8Uint64x2(xbroadcast1To8Uint64x2 archsimd.Uint64x2) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return archsimd.BroadcastUint64x8(xbroadcast1To8Uint64x2.GetElem(0))
}

//go:noinline
func llvmShuffleGetHiFloat32x16(xGetHiFloat32x16 archsimd.Float32x16) archsimd.Float32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x8{}
	}
	return xGetHiFloat32x16.GetHi()
}

//go:noinline
func llvmShuffleGetHiFloat32x8(xGetHiFloat32x8 archsimd.Float32x8) archsimd.Float32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x4{}
	}
	return xGetHiFloat32x8.GetHi()
}

//go:noinline
func llvmShuffleGetHiFloat64x4(xGetHiFloat64x4 archsimd.Float64x4) archsimd.Float64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x2{}
	}
	return xGetHiFloat64x4.GetHi()
}

//go:noinline
func llvmShuffleGetHiFloat64x8(xGetHiFloat64x8 archsimd.Float64x8) archsimd.Float64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x4{}
	}
	return xGetHiFloat64x8.GetHi()
}

//go:noinline
func llvmShuffleGetHiInt16x16(xGetHiInt16x16 archsimd.Int16x16) archsimd.Int16x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x8{}
	}
	return xGetHiInt16x16.GetHi()
}

//go:noinline
func llvmShuffleGetHiInt16x32(xGetHiInt16x32 archsimd.Int16x32) archsimd.Int16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x16{}
	}
	return xGetHiInt16x32.GetHi()
}

//go:noinline
func llvmShuffleGetHiInt32x16(xGetHiInt32x16 archsimd.Int32x16) archsimd.Int32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x8{}
	}
	return xGetHiInt32x16.GetHi()
}

//go:noinline
func llvmShuffleGetHiInt32x8(xGetHiInt32x8 archsimd.Int32x8) archsimd.Int32x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x4{}
	}
	return xGetHiInt32x8.GetHi()
}

//go:noinline
func llvmShuffleGetHiInt64x4(xGetHiInt64x4 archsimd.Int64x4) archsimd.Int64x2 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x2{}
	}
	return xGetHiInt64x4.GetHi()
}

//go:noinline
func llvmShuffleGetHiInt64x8(xGetHiInt64x8 archsimd.Int64x8) archsimd.Int64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x4{}
	}
	return xGetHiInt64x8.GetHi()
}

//go:noinline
func llvmShuffleGetHiInt8x32(xGetHiInt8x32 archsimd.Int8x32) archsimd.Int8x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int8x16{}
	}
	return xGetHiInt8x32.GetHi()
}

//go:noinline
func llvmShuffleGetHiInt8x64(xGetHiInt8x64 archsimd.Int8x64) archsimd.Int8x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x32{}
	}
	return xGetHiInt8x64.GetHi()
}

//go:noinline
func llvmShuffleGetHiUint16x16(xGetHiUint16x16 archsimd.Uint16x16) archsimd.Uint16x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x8{}
	}
	return xGetHiUint16x16.GetHi()
}

//go:noinline
func llvmShuffleGetHiUint16x32(xGetHiUint16x32 archsimd.Uint16x32) archsimd.Uint16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x16{}
	}
	return xGetHiUint16x32.GetHi()
}

//go:noinline
func llvmShuffleGetHiUint32x16(xGetHiUint32x16 archsimd.Uint32x16) archsimd.Uint32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x8{}
	}
	return xGetHiUint32x16.GetHi()
}

//go:noinline
func llvmShuffleGetHiUint32x8(xGetHiUint32x8 archsimd.Uint32x8) archsimd.Uint32x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x4{}
	}
	return xGetHiUint32x8.GetHi()
}

//go:noinline
func llvmShuffleGetHiUint64x4(xGetHiUint64x4 archsimd.Uint64x4) archsimd.Uint64x2 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x2{}
	}
	return xGetHiUint64x4.GetHi()
}

//go:noinline
func llvmShuffleGetHiUint64x8(xGetHiUint64x8 archsimd.Uint64x8) archsimd.Uint64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x4{}
	}
	return xGetHiUint64x8.GetHi()
}

//go:noinline
func llvmShuffleGetHiUint8x32(xGetHiUint8x32 archsimd.Uint8x32) archsimd.Uint8x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x16{}
	}
	return xGetHiUint8x32.GetHi()
}

//go:noinline
func llvmShuffleGetHiUint8x64(xGetHiUint8x64 archsimd.Uint8x64) archsimd.Uint8x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x32{}
	}
	return xGetHiUint8x64.GetHi()
}

//go:noinline
func llvmShuffleGetLoFloat32x16(xGetLoFloat32x16 archsimd.Float32x16) archsimd.Float32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x8{}
	}
	return xGetLoFloat32x16.GetLo()
}

//go:noinline
func llvmShuffleGetLoFloat32x8(xGetLoFloat32x8 archsimd.Float32x8) archsimd.Float32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x4{}
	}
	return xGetLoFloat32x8.GetLo()
}

//go:noinline
func llvmShuffleGetLoFloat64x4(xGetLoFloat64x4 archsimd.Float64x4) archsimd.Float64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x2{}
	}
	return xGetLoFloat64x4.GetLo()
}

//go:noinline
func llvmShuffleGetLoFloat64x8(xGetLoFloat64x8 archsimd.Float64x8) archsimd.Float64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x4{}
	}
	return xGetLoFloat64x8.GetLo()
}

//go:noinline
func llvmShuffleGetLoInt16x16(xGetLoInt16x16 archsimd.Int16x16) archsimd.Int16x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x8{}
	}
	return xGetLoInt16x16.GetLo()
}

//go:noinline
func llvmShuffleGetLoInt16x32(xGetLoInt16x32 archsimd.Int16x32) archsimd.Int16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x16{}
	}
	return xGetLoInt16x32.GetLo()
}

//go:noinline
func llvmShuffleGetLoInt32x16(xGetLoInt32x16 archsimd.Int32x16) archsimd.Int32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x8{}
	}
	return xGetLoInt32x16.GetLo()
}

//go:noinline
func llvmShuffleGetLoInt32x8(xGetLoInt32x8 archsimd.Int32x8) archsimd.Int32x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x4{}
	}
	return xGetLoInt32x8.GetLo()
}

//go:noinline
func llvmShuffleGetLoInt64x4(xGetLoInt64x4 archsimd.Int64x4) archsimd.Int64x2 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x2{}
	}
	return xGetLoInt64x4.GetLo()
}

//go:noinline
func llvmShuffleGetLoInt64x8(xGetLoInt64x8 archsimd.Int64x8) archsimd.Int64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x4{}
	}
	return xGetLoInt64x8.GetLo()
}

//go:noinline
func llvmShuffleGetLoInt8x32(xGetLoInt8x32 archsimd.Int8x32) archsimd.Int8x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int8x16{}
	}
	return xGetLoInt8x32.GetLo()
}

//go:noinline
func llvmShuffleGetLoInt8x64(xGetLoInt8x64 archsimd.Int8x64) archsimd.Int8x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x32{}
	}
	return xGetLoInt8x64.GetLo()
}

//go:noinline
func llvmShuffleGetLoUint16x16(xGetLoUint16x16 archsimd.Uint16x16) archsimd.Uint16x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x8{}
	}
	return xGetLoUint16x16.GetLo()
}

//go:noinline
func llvmShuffleGetLoUint16x32(xGetLoUint16x32 archsimd.Uint16x32) archsimd.Uint16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x16{}
	}
	return xGetLoUint16x32.GetLo()
}

//go:noinline
func llvmShuffleGetLoUint32x16(xGetLoUint32x16 archsimd.Uint32x16) archsimd.Uint32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x8{}
	}
	return xGetLoUint32x16.GetLo()
}

//go:noinline
func llvmShuffleGetLoUint32x8(xGetLoUint32x8 archsimd.Uint32x8) archsimd.Uint32x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x4{}
	}
	return xGetLoUint32x8.GetLo()
}

//go:noinline
func llvmShuffleGetLoUint64x4(xGetLoUint64x4 archsimd.Uint64x4) archsimd.Uint64x2 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x2{}
	}
	return xGetLoUint64x4.GetLo()
}

//go:noinline
func llvmShuffleGetLoUint64x8(xGetLoUint64x8 archsimd.Uint64x8) archsimd.Uint64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x4{}
	}
	return xGetLoUint64x8.GetLo()
}

//go:noinline
func llvmShuffleGetLoUint8x32(xGetLoUint8x32 archsimd.Uint8x32) archsimd.Uint8x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x16{}
	}
	return xGetLoUint8x32.GetLo()
}

//go:noinline
func llvmShuffleGetLoUint8x64(xGetLoUint8x64 archsimd.Uint8x64) archsimd.Uint8x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x32{}
	}
	return xGetLoUint8x64.GetLo()
}

//go:noinline
func llvmShuffleInterleaveHiGroupedInt16x16(xInterleaveHiGroupedInt16x16 archsimd.Int16x16, yInterleaveHiGroupedInt16x16 archsimd.Int16x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return xInterleaveHiGroupedInt16x16.InterleaveHiGrouped(yInterleaveHiGroupedInt16x16)
}

//go:noinline
func llvmShuffleInterleaveHiGroupedInt16x32(xInterleaveHiGroupedInt16x32 archsimd.Int16x32, yInterleaveHiGroupedInt16x32 archsimd.Int16x32) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return xInterleaveHiGroupedInt16x32.InterleaveHiGrouped(yInterleaveHiGroupedInt16x32)
}

//go:noinline
func llvmShuffleInterleaveHiGroupedInt32x16(xInterleaveHiGroupedInt32x16 archsimd.Int32x16, yInterleaveHiGroupedInt32x16 archsimd.Int32x16) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return xInterleaveHiGroupedInt32x16.InterleaveHiGrouped(yInterleaveHiGroupedInt32x16)
}

//go:noinline
func llvmShuffleInterleaveHiGroupedInt32x8(xInterleaveHiGroupedInt32x8 archsimd.Int32x8, yInterleaveHiGroupedInt32x8 archsimd.Int32x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return xInterleaveHiGroupedInt32x8.InterleaveHiGrouped(yInterleaveHiGroupedInt32x8)
}

//go:noinline
func llvmShuffleInterleaveHiGroupedInt64x4(xInterleaveHiGroupedInt64x4 archsimd.Int64x4, yInterleaveHiGroupedInt64x4 archsimd.Int64x4) archsimd.Int64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x4{}
	}
	return xInterleaveHiGroupedInt64x4.InterleaveHiGrouped(yInterleaveHiGroupedInt64x4)
}

//go:noinline
func llvmShuffleInterleaveHiGroupedInt64x8(xInterleaveHiGroupedInt64x8 archsimd.Int64x8, yInterleaveHiGroupedInt64x8 archsimd.Int64x8) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return xInterleaveHiGroupedInt64x8.InterleaveHiGrouped(yInterleaveHiGroupedInt64x8)
}

//go:noinline
func llvmShuffleInterleaveHiGroupedUint16x16(xInterleaveHiGroupedUint16x16 archsimd.Uint16x16, yInterleaveHiGroupedUint16x16 archsimd.Uint16x16) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return xInterleaveHiGroupedUint16x16.InterleaveHiGrouped(yInterleaveHiGroupedUint16x16)
}

//go:noinline
func llvmShuffleInterleaveHiGroupedUint16x32(xInterleaveHiGroupedUint16x32 archsimd.Uint16x32, yInterleaveHiGroupedUint16x32 archsimd.Uint16x32) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return xInterleaveHiGroupedUint16x32.InterleaveHiGrouped(yInterleaveHiGroupedUint16x32)
}

//go:noinline
func llvmShuffleInterleaveHiGroupedUint32x16(xInterleaveHiGroupedUint32x16 archsimd.Uint32x16, yInterleaveHiGroupedUint32x16 archsimd.Uint32x16) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return xInterleaveHiGroupedUint32x16.InterleaveHiGrouped(yInterleaveHiGroupedUint32x16)
}

//go:noinline
func llvmShuffleInterleaveHiGroupedUint32x8(xInterleaveHiGroupedUint32x8 archsimd.Uint32x8, yInterleaveHiGroupedUint32x8 archsimd.Uint32x8) archsimd.Uint32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x8{}
	}
	return xInterleaveHiGroupedUint32x8.InterleaveHiGrouped(yInterleaveHiGroupedUint32x8)
}

//go:noinline
func llvmShuffleInterleaveHiGroupedUint64x4(xInterleaveHiGroupedUint64x4 archsimd.Uint64x4, yInterleaveHiGroupedUint64x4 archsimd.Uint64x4) archsimd.Uint64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x4{}
	}
	return xInterleaveHiGroupedUint64x4.InterleaveHiGrouped(yInterleaveHiGroupedUint64x4)
}

//go:noinline
func llvmShuffleInterleaveHiGroupedUint64x8(xInterleaveHiGroupedUint64x8 archsimd.Uint64x8, yInterleaveHiGroupedUint64x8 archsimd.Uint64x8) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return xInterleaveHiGroupedUint64x8.InterleaveHiGrouped(yInterleaveHiGroupedUint64x8)
}

//go:noinline
func llvmShuffleInterleaveHiInt16x8(xInterleaveHiInt16x8 archsimd.Int16x8, yInterleaveHiInt16x8 archsimd.Int16x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Int16x8{}
	}
	return xInterleaveHiInt16x8.InterleaveHi(yInterleaveHiInt16x8)
}

//go:noinline
func llvmShuffleInterleaveHiInt32x4(xInterleaveHiInt32x4 archsimd.Int32x4, yInterleaveHiInt32x4 archsimd.Int32x4) archsimd.Int32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x4{}
	}
	return xInterleaveHiInt32x4.InterleaveHi(yInterleaveHiInt32x4)
}

//go:noinline
func llvmShuffleInterleaveHiInt64x2(xInterleaveHiInt64x2 archsimd.Int64x2, yInterleaveHiInt64x2 archsimd.Int64x2) archsimd.Int64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Int64x2{}
	}
	return xInterleaveHiInt64x2.InterleaveHi(yInterleaveHiInt64x2)
}

//go:noinline
func llvmShuffleInterleaveHiUint16x8(xInterleaveHiUint16x8 archsimd.Uint16x8, yInterleaveHiUint16x8 archsimd.Uint16x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint16x8{}
	}
	return xInterleaveHiUint16x8.InterleaveHi(yInterleaveHiUint16x8)
}

//go:noinline
func llvmShuffleInterleaveHiUint32x4(xInterleaveHiUint32x4 archsimd.Uint32x4, yInterleaveHiUint32x4 archsimd.Uint32x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint32x4{}
	}
	return xInterleaveHiUint32x4.InterleaveHi(yInterleaveHiUint32x4)
}

//go:noinline
func llvmShuffleInterleaveHiUint64x2(xInterleaveHiUint64x2 archsimd.Uint64x2, yInterleaveHiUint64x2 archsimd.Uint64x2) archsimd.Uint64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint64x2{}
	}
	return xInterleaveHiUint64x2.InterleaveHi(yInterleaveHiUint64x2)
}

//go:noinline
func llvmShuffleInterleaveLoGroupedInt16x16(xInterleaveLoGroupedInt16x16 archsimd.Int16x16, yInterleaveLoGroupedInt16x16 archsimd.Int16x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return xInterleaveLoGroupedInt16x16.InterleaveLoGrouped(yInterleaveLoGroupedInt16x16)
}

//go:noinline
func llvmShuffleInterleaveLoGroupedInt16x32(xInterleaveLoGroupedInt16x32 archsimd.Int16x32, yInterleaveLoGroupedInt16x32 archsimd.Int16x32) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return xInterleaveLoGroupedInt16x32.InterleaveLoGrouped(yInterleaveLoGroupedInt16x32)
}

//go:noinline
func llvmShuffleInterleaveLoGroupedInt32x16(xInterleaveLoGroupedInt32x16 archsimd.Int32x16, yInterleaveLoGroupedInt32x16 archsimd.Int32x16) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return xInterleaveLoGroupedInt32x16.InterleaveLoGrouped(yInterleaveLoGroupedInt32x16)
}

//go:noinline
func llvmShuffleInterleaveLoGroupedInt32x8(xInterleaveLoGroupedInt32x8 archsimd.Int32x8, yInterleaveLoGroupedInt32x8 archsimd.Int32x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return xInterleaveLoGroupedInt32x8.InterleaveLoGrouped(yInterleaveLoGroupedInt32x8)
}

//go:noinline
func llvmShuffleInterleaveLoGroupedInt64x4(xInterleaveLoGroupedInt64x4 archsimd.Int64x4, yInterleaveLoGroupedInt64x4 archsimd.Int64x4) archsimd.Int64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x4{}
	}
	return xInterleaveLoGroupedInt64x4.InterleaveLoGrouped(yInterleaveLoGroupedInt64x4)
}

//go:noinline
func llvmShuffleInterleaveLoGroupedInt64x8(xInterleaveLoGroupedInt64x8 archsimd.Int64x8, yInterleaveLoGroupedInt64x8 archsimd.Int64x8) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return xInterleaveLoGroupedInt64x8.InterleaveLoGrouped(yInterleaveLoGroupedInt64x8)
}

//go:noinline
func llvmShuffleInterleaveLoGroupedUint16x16(xInterleaveLoGroupedUint16x16 archsimd.Uint16x16, yInterleaveLoGroupedUint16x16 archsimd.Uint16x16) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return xInterleaveLoGroupedUint16x16.InterleaveLoGrouped(yInterleaveLoGroupedUint16x16)
}

//go:noinline
func llvmShuffleInterleaveLoGroupedUint16x32(xInterleaveLoGroupedUint16x32 archsimd.Uint16x32, yInterleaveLoGroupedUint16x32 archsimd.Uint16x32) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return xInterleaveLoGroupedUint16x32.InterleaveLoGrouped(yInterleaveLoGroupedUint16x32)
}

//go:noinline
func llvmShuffleInterleaveLoGroupedUint32x16(xInterleaveLoGroupedUint32x16 archsimd.Uint32x16, yInterleaveLoGroupedUint32x16 archsimd.Uint32x16) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return xInterleaveLoGroupedUint32x16.InterleaveLoGrouped(yInterleaveLoGroupedUint32x16)
}

//go:noinline
func llvmShuffleInterleaveLoGroupedUint32x8(xInterleaveLoGroupedUint32x8 archsimd.Uint32x8, yInterleaveLoGroupedUint32x8 archsimd.Uint32x8) archsimd.Uint32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x8{}
	}
	return xInterleaveLoGroupedUint32x8.InterleaveLoGrouped(yInterleaveLoGroupedUint32x8)
}

//go:noinline
func llvmShuffleInterleaveLoGroupedUint64x4(xInterleaveLoGroupedUint64x4 archsimd.Uint64x4, yInterleaveLoGroupedUint64x4 archsimd.Uint64x4) archsimd.Uint64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x4{}
	}
	return xInterleaveLoGroupedUint64x4.InterleaveLoGrouped(yInterleaveLoGroupedUint64x4)
}

//go:noinline
func llvmShuffleInterleaveLoGroupedUint64x8(xInterleaveLoGroupedUint64x8 archsimd.Uint64x8, yInterleaveLoGroupedUint64x8 archsimd.Uint64x8) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return xInterleaveLoGroupedUint64x8.InterleaveLoGrouped(yInterleaveLoGroupedUint64x8)
}

//go:noinline
func llvmShuffleInterleaveLoInt16x8(xInterleaveLoInt16x8 archsimd.Int16x8, yInterleaveLoInt16x8 archsimd.Int16x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Int16x8{}
	}
	return xInterleaveLoInt16x8.InterleaveLo(yInterleaveLoInt16x8)
}

//go:noinline
func llvmShuffleInterleaveLoInt32x4(xInterleaveLoInt32x4 archsimd.Int32x4, yInterleaveLoInt32x4 archsimd.Int32x4) archsimd.Int32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x4{}
	}
	return xInterleaveLoInt32x4.InterleaveLo(yInterleaveLoInt32x4)
}

//go:noinline
func llvmShuffleInterleaveLoInt64x2(xInterleaveLoInt64x2 archsimd.Int64x2, yInterleaveLoInt64x2 archsimd.Int64x2) archsimd.Int64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Int64x2{}
	}
	return xInterleaveLoInt64x2.InterleaveLo(yInterleaveLoInt64x2)
}

//go:noinline
func llvmShuffleInterleaveLoUint16x8(xInterleaveLoUint16x8 archsimd.Uint16x8, yInterleaveLoUint16x8 archsimd.Uint16x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint16x8{}
	}
	return xInterleaveLoUint16x8.InterleaveLo(yInterleaveLoUint16x8)
}

//go:noinline
func llvmShuffleInterleaveLoUint32x4(xInterleaveLoUint32x4 archsimd.Uint32x4, yInterleaveLoUint32x4 archsimd.Uint32x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint32x4{}
	}
	return xInterleaveLoUint32x4.InterleaveLo(yInterleaveLoUint32x4)
}

//go:noinline
func llvmShuffleInterleaveLoUint64x2(xInterleaveLoUint64x2 archsimd.Uint64x2, yInterleaveLoUint64x2 archsimd.Uint64x2) archsimd.Uint64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint64x2{}
	}
	return xInterleaveLoUint64x2.InterleaveLo(yInterleaveLoUint64x2)
}

//go:noinline
func llvmShuffleSetHiFloat32x16(xSetHiFloat32x16 archsimd.Float32x16, ySetHiFloat32x16 archsimd.Float32x8) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return xSetHiFloat32x16.SetHi(ySetHiFloat32x16)
}

//go:noinline
func llvmShuffleSetHiFloat32x8(xSetHiFloat32x8 archsimd.Float32x8, ySetHiFloat32x8 archsimd.Float32x4) archsimd.Float32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x8{}
	}
	return xSetHiFloat32x8.SetHi(ySetHiFloat32x8)
}

//go:noinline
func llvmShuffleSetHiFloat64x4(xSetHiFloat64x4 archsimd.Float64x4, ySetHiFloat64x4 archsimd.Float64x2) archsimd.Float64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x4{}
	}
	return xSetHiFloat64x4.SetHi(ySetHiFloat64x4)
}

//go:noinline
func llvmShuffleSetHiFloat64x8(xSetHiFloat64x8 archsimd.Float64x8, ySetHiFloat64x8 archsimd.Float64x4) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return xSetHiFloat64x8.SetHi(ySetHiFloat64x8)
}

//go:noinline
func llvmShuffleSetHiInt16x16(xSetHiInt16x16 archsimd.Int16x16, ySetHiInt16x16 archsimd.Int16x8) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return xSetHiInt16x16.SetHi(ySetHiInt16x16)
}

//go:noinline
func llvmShuffleSetHiInt16x32(xSetHiInt16x32 archsimd.Int16x32, ySetHiInt16x32 archsimd.Int16x16) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return xSetHiInt16x32.SetHi(ySetHiInt16x32)
}

//go:noinline
func llvmShuffleSetHiInt32x16(xSetHiInt32x16 archsimd.Int32x16, ySetHiInt32x16 archsimd.Int32x8) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return xSetHiInt32x16.SetHi(ySetHiInt32x16)
}

//go:noinline
func llvmShuffleSetHiInt32x8(xSetHiInt32x8 archsimd.Int32x8, ySetHiInt32x8 archsimd.Int32x4) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return xSetHiInt32x8.SetHi(ySetHiInt32x8)
}

//go:noinline
func llvmShuffleSetHiInt64x4(xSetHiInt64x4 archsimd.Int64x4, ySetHiInt64x4 archsimd.Int64x2) archsimd.Int64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x4{}
	}
	return xSetHiInt64x4.SetHi(ySetHiInt64x4)
}

//go:noinline
func llvmShuffleSetHiInt64x8(xSetHiInt64x8 archsimd.Int64x8, ySetHiInt64x8 archsimd.Int64x4) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return xSetHiInt64x8.SetHi(ySetHiInt64x8)
}

//go:noinline
func llvmShuffleSetHiInt8x32(xSetHiInt8x32 archsimd.Int8x32, ySetHiInt8x32 archsimd.Int8x16) archsimd.Int8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int8x32{}
	}
	return xSetHiInt8x32.SetHi(ySetHiInt8x32)
}

//go:noinline
func llvmShuffleSetHiInt8x64(xSetHiInt8x64 archsimd.Int8x64, ySetHiInt8x64 archsimd.Int8x32) archsimd.Int8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x64{}
	}
	return xSetHiInt8x64.SetHi(ySetHiInt8x64)
}

//go:noinline
func llvmShuffleSetHiUint16x16(xSetHiUint16x16 archsimd.Uint16x16, ySetHiUint16x16 archsimd.Uint16x8) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return xSetHiUint16x16.SetHi(ySetHiUint16x16)
}

//go:noinline
func llvmShuffleSetHiUint16x32(xSetHiUint16x32 archsimd.Uint16x32, ySetHiUint16x32 archsimd.Uint16x16) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return xSetHiUint16x32.SetHi(ySetHiUint16x32)
}

//go:noinline
func llvmShuffleSetHiUint32x16(xSetHiUint32x16 archsimd.Uint32x16, ySetHiUint32x16 archsimd.Uint32x8) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return xSetHiUint32x16.SetHi(ySetHiUint32x16)
}

//go:noinline
func llvmShuffleSetHiUint32x8(xSetHiUint32x8 archsimd.Uint32x8, ySetHiUint32x8 archsimd.Uint32x4) archsimd.Uint32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x8{}
	}
	return xSetHiUint32x8.SetHi(ySetHiUint32x8)
}

//go:noinline
func llvmShuffleSetHiUint64x4(xSetHiUint64x4 archsimd.Uint64x4, ySetHiUint64x4 archsimd.Uint64x2) archsimd.Uint64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x4{}
	}
	return xSetHiUint64x4.SetHi(ySetHiUint64x4)
}

//go:noinline
func llvmShuffleSetHiUint64x8(xSetHiUint64x8 archsimd.Uint64x8, ySetHiUint64x8 archsimd.Uint64x4) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return xSetHiUint64x8.SetHi(ySetHiUint64x8)
}

//go:noinline
func llvmShuffleSetHiUint8x32(xSetHiUint8x32 archsimd.Uint8x32, ySetHiUint8x32 archsimd.Uint8x16) archsimd.Uint8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x32{}
	}
	return xSetHiUint8x32.SetHi(ySetHiUint8x32)
}

//go:noinline
func llvmShuffleSetHiUint8x64(xSetHiUint8x64 archsimd.Uint8x64, ySetHiUint8x64 archsimd.Uint8x32) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x64{}
	}
	return xSetHiUint8x64.SetHi(ySetHiUint8x64)
}

//go:noinline
func llvmShuffleSetLoFloat32x16(xSetLoFloat32x16 archsimd.Float32x16, ySetLoFloat32x16 archsimd.Float32x8) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return xSetLoFloat32x16.SetLo(ySetLoFloat32x16)
}

//go:noinline
func llvmShuffleSetLoFloat32x8(xSetLoFloat32x8 archsimd.Float32x8, ySetLoFloat32x8 archsimd.Float32x4) archsimd.Float32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x8{}
	}
	return xSetLoFloat32x8.SetLo(ySetLoFloat32x8)
}

//go:noinline
func llvmShuffleSetLoFloat64x4(xSetLoFloat64x4 archsimd.Float64x4, ySetLoFloat64x4 archsimd.Float64x2) archsimd.Float64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x4{}
	}
	return xSetLoFloat64x4.SetLo(ySetLoFloat64x4)
}

//go:noinline
func llvmShuffleSetLoFloat64x8(xSetLoFloat64x8 archsimd.Float64x8, ySetLoFloat64x8 archsimd.Float64x4) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return xSetLoFloat64x8.SetLo(ySetLoFloat64x8)
}

//go:noinline
func llvmShuffleSetLoInt16x16(xSetLoInt16x16 archsimd.Int16x16, ySetLoInt16x16 archsimd.Int16x8) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return xSetLoInt16x16.SetLo(ySetLoInt16x16)
}

//go:noinline
func llvmShuffleSetLoInt16x32(xSetLoInt16x32 archsimd.Int16x32, ySetLoInt16x32 archsimd.Int16x16) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return xSetLoInt16x32.SetLo(ySetLoInt16x32)
}

//go:noinline
func llvmShuffleSetLoInt32x16(xSetLoInt32x16 archsimd.Int32x16, ySetLoInt32x16 archsimd.Int32x8) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return xSetLoInt32x16.SetLo(ySetLoInt32x16)
}

//go:noinline
func llvmShuffleSetLoInt32x8(xSetLoInt32x8 archsimd.Int32x8, ySetLoInt32x8 archsimd.Int32x4) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return xSetLoInt32x8.SetLo(ySetLoInt32x8)
}

//go:noinline
func llvmShuffleSetLoInt64x4(xSetLoInt64x4 archsimd.Int64x4, ySetLoInt64x4 archsimd.Int64x2) archsimd.Int64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x4{}
	}
	return xSetLoInt64x4.SetLo(ySetLoInt64x4)
}

//go:noinline
func llvmShuffleSetLoInt64x8(xSetLoInt64x8 archsimd.Int64x8, ySetLoInt64x8 archsimd.Int64x4) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return xSetLoInt64x8.SetLo(ySetLoInt64x8)
}

//go:noinline
func llvmShuffleSetLoInt8x32(xSetLoInt8x32 archsimd.Int8x32, ySetLoInt8x32 archsimd.Int8x16) archsimd.Int8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int8x32{}
	}
	return xSetLoInt8x32.SetLo(ySetLoInt8x32)
}

//go:noinline
func llvmShuffleSetLoInt8x64(xSetLoInt8x64 archsimd.Int8x64, ySetLoInt8x64 archsimd.Int8x32) archsimd.Int8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x64{}
	}
	return xSetLoInt8x64.SetLo(ySetLoInt8x64)
}

//go:noinline
func llvmShuffleSetLoUint16x16(xSetLoUint16x16 archsimd.Uint16x16, ySetLoUint16x16 archsimd.Uint16x8) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return xSetLoUint16x16.SetLo(ySetLoUint16x16)
}

//go:noinline
func llvmShuffleSetLoUint16x32(xSetLoUint16x32 archsimd.Uint16x32, ySetLoUint16x32 archsimd.Uint16x16) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return xSetLoUint16x32.SetLo(ySetLoUint16x32)
}

//go:noinline
func llvmShuffleSetLoUint32x16(xSetLoUint32x16 archsimd.Uint32x16, ySetLoUint32x16 archsimd.Uint32x8) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return xSetLoUint32x16.SetLo(ySetLoUint32x16)
}

//go:noinline
func llvmShuffleSetLoUint32x8(xSetLoUint32x8 archsimd.Uint32x8, ySetLoUint32x8 archsimd.Uint32x4) archsimd.Uint32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x8{}
	}
	return xSetLoUint32x8.SetLo(ySetLoUint32x8)
}

//go:noinline
func llvmShuffleSetLoUint64x4(xSetLoUint64x4 archsimd.Uint64x4, ySetLoUint64x4 archsimd.Uint64x2) archsimd.Uint64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x4{}
	}
	return xSetLoUint64x4.SetLo(ySetLoUint64x4)
}

//go:noinline
func llvmShuffleSetLoUint64x8(xSetLoUint64x8 archsimd.Uint64x8, ySetLoUint64x8 archsimd.Uint64x4) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return xSetLoUint64x8.SetLo(ySetLoUint64x8)
}

//go:noinline
func llvmShuffleSetLoUint8x32(xSetLoUint8x32 archsimd.Uint8x32, ySetLoUint8x32 archsimd.Uint8x16) archsimd.Uint8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x32{}
	}
	return xSetLoUint8x32.SetLo(ySetLoUint8x32)
}

//go:noinline
func llvmShuffleSetLoUint8x64(xSetLoUint8x64 archsimd.Uint8x64, ySetLoUint8x64 archsimd.Uint8x32) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x64{}
	}
	return xSetLoUint8x64.SetLo(ySetLoUint8x64)
}
