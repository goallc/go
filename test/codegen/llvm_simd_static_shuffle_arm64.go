// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && arm64

package codegen

import "simd/archsimd"

// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShufflebroadcast1To16Int8x16(SB)
// LLVM-ASM-ARM64: VDUP V0.B[0], V0.B16
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShufflebroadcast1To16Uint8x16(SB)
// LLVM-ASM-ARM64: VDUP V0.B[0], V0.B16
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShufflebroadcast1To2Float64x2(SB)
// LLVM-ASM-ARM64: VDUP V0.D[0], V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShufflebroadcast1To2Int64x2(SB)
// LLVM-ASM-ARM64: VDUP V0.D[0], V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShufflebroadcast1To2Uint64x2(SB)
// LLVM-ASM-ARM64: VDUP V0.D[0], V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShufflebroadcast1To4Float32x4(SB)
// LLVM-ASM-ARM64: VDUP V0.S[0], V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShufflebroadcast1To4Int32x4(SB)
// LLVM-ASM-ARM64: VDUP V0.S[0], V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShufflebroadcast1To4Uint32x4(SB)
// LLVM-ASM-ARM64: VDUP V0.S[0], V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShufflebroadcast1To8Int16x8(SB)
// LLVM-ASM-ARM64: VDUP V0.H[0], V0.H8
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShufflebroadcast1To8Uint16x8(SB)
// LLVM-ASM-ARM64: VDUP V0.H[0], V0.H8
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleConcatEvenInt16x8(SB)
// LLVM-ASM-ARM64: VUZP1 V1.H8, V0.H8, V0.H8
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleConcatEvenInt32x4(SB)
// LLVM-ASM-ARM64: VUZP1 V1.S4, V0.S4, V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleConcatEvenInt64x2(SB)
// LLVM-ASM-ARM64: VZIP1 V1.D2, V0.D2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleConcatEvenInt8x16(SB)
// LLVM-ASM-ARM64: VUZP1 V1.B16, V0.B16, V0.B16
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleConcatEvenUint16x8(SB)
// LLVM-ASM-ARM64: VUZP1 V1.H8, V0.H8, V0.H8
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleConcatEvenUint32x4(SB)
// LLVM-ASM-ARM64: VUZP1 V1.S4, V0.S4, V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleConcatEvenUint64x2(SB)
// LLVM-ASM-ARM64: VZIP1 V1.D2, V0.D2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleConcatEvenUint8x16(SB)
// LLVM-ASM-ARM64: VUZP1 V1.B16, V0.B16, V0.B16
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleConcatOddInt16x8(SB)
// LLVM-ASM-ARM64: VUZP2 V1.H8, V0.H8, V0.H8
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleConcatOddInt32x4(SB)
// LLVM-ASM-ARM64: VUZP2 V1.S4, V0.S4, V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleConcatOddInt64x2(SB)
// LLVM-ASM-ARM64: VZIP2 V1.D2, V0.D2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleConcatOddInt8x16(SB)
// LLVM-ASM-ARM64: VUZP2 V1.B16, V0.B16, V0.B16
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleConcatOddUint16x8(SB)
// LLVM-ASM-ARM64: VUZP2 V1.H8, V0.H8, V0.H8
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleConcatOddUint32x4(SB)
// LLVM-ASM-ARM64: VUZP2 V1.S4, V0.S4, V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleConcatOddUint64x2(SB)
// LLVM-ASM-ARM64: VZIP2 V1.D2, V0.D2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleConcatOddUint8x16(SB)
// LLVM-ASM-ARM64: VUZP2 V1.B16, V0.B16, V0.B16
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveEvenInt16x8(SB)
// LLVM-ASM-ARM64: VTRN1 V1.H8, V0.H8, V0.H8
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveEvenInt32x4(SB)
// LLVM-ASM-ARM64: VTRN1 V1.S4, V0.S4, V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveEvenInt64x2(SB)
// LLVM-ASM-ARM64: VZIP1 V1.D2, V0.D2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveEvenInt8x16(SB)
// LLVM-ASM-ARM64: VTRN1 V1.B16, V0.B16, V0.B16
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveEvenUint16x8(SB)
// LLVM-ASM-ARM64: VTRN1 V1.H8, V0.H8, V0.H8
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveEvenUint32x4(SB)
// LLVM-ASM-ARM64: VTRN1 V1.S4, V0.S4, V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveEvenUint64x2(SB)
// LLVM-ASM-ARM64: VZIP1 V1.D2, V0.D2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveEvenUint8x16(SB)
// LLVM-ASM-ARM64: VTRN1 V1.B16, V0.B16, V0.B16
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveHiInt16x8(SB)
// LLVM-ASM-ARM64: VZIP2 V1.H8, V0.H8, V0.H8
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveHiInt32x4(SB)
// LLVM-ASM-ARM64: VZIP2 V1.S4, V0.S4, V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveHiInt64x2(SB)
// LLVM-ASM-ARM64: VZIP2 V1.D2, V0.D2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveHiInt8x16(SB)
// LLVM-ASM-ARM64: VZIP2 V1.B16, V0.B16, V0.B16
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveHiUint16x8(SB)
// LLVM-ASM-ARM64: VZIP2 V1.H8, V0.H8, V0.H8
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveHiUint32x4(SB)
// LLVM-ASM-ARM64: VZIP2 V1.S4, V0.S4, V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveHiUint64x2(SB)
// LLVM-ASM-ARM64: VZIP2 V1.D2, V0.D2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveHiUint8x16(SB)
// LLVM-ASM-ARM64: VZIP2 V1.B16, V0.B16, V0.B16
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveLoInt16x8(SB)
// LLVM-ASM-ARM64: VZIP1 V1.H8, V0.H8, V0.H8
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveLoInt32x4(SB)
// LLVM-ASM-ARM64: VZIP1 V1.S4, V0.S4, V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveLoInt64x2(SB)
// LLVM-ASM-ARM64: VZIP1 V1.D2, V0.D2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveLoInt8x16(SB)
// LLVM-ASM-ARM64: VZIP1 V1.B16, V0.B16, V0.B16
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveLoUint16x8(SB)
// LLVM-ASM-ARM64: VZIP1 V1.H8, V0.H8, V0.H8
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveLoUint32x4(SB)
// LLVM-ASM-ARM64: VZIP1 V1.S4, V0.S4, V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveLoUint64x2(SB)
// LLVM-ASM-ARM64: VZIP1 V1.D2, V0.D2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveLoUint8x16(SB)
// LLVM-ASM-ARM64: VZIP1 V1.B16, V0.B16, V0.B16
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveOddInt16x8(SB)
// LLVM-ASM-ARM64: VTRN2 V1.H8, V0.H8, V0.H8
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveOddInt32x4(SB)
// LLVM-ASM-ARM64: VTRN2 V1.S4, V0.S4, V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveOddInt64x2(SB)
// LLVM-ASM-ARM64: VZIP2 V1.D2, V0.D2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveOddInt8x16(SB)
// LLVM-ASM-ARM64: VTRN2 V1.B16, V0.B16, V0.B16
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveOddUint16x8(SB)
// LLVM-ASM-ARM64: VTRN2 V1.H8, V0.H8, V0.H8
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveOddUint32x4(SB)
// LLVM-ASM-ARM64: VTRN2 V1.S4, V0.S4, V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveOddUint64x2(SB)
// LLVM-ASM-ARM64: VZIP2 V1.D2, V0.D2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmShuffleInterleaveOddUint8x16(SB)
// LLVM-ASM-ARM64: VTRN2 V1.B16, V0.B16, V0.B16
// LLVM-ASM-ARM64-NEXT: {{.*}}RET

// Unique parameter names tie each routing check to its API without depending
// on function emission order under parallel compilation.
// LLVM-ARM64-DAG: define {{.*}} <16 x i8> @codegen.llvmShufflebroadcast1To16Int8x16(<16 x i8> %xbroadcast1To16Int8x16
// LLVM-ARM64-DAG: extractelement <16 x i8> %xbroadcast1To16Int8x16, i32 0
// LLVM-ARM64-DAG: shufflevector <16 x i8> {{%[^,]+}}, <16 x i8> zeroinitializer, <16 x i32> zeroinitializer
// LLVM-ARM64-DAG: define {{.*}} <16 x i8> @codegen.llvmShufflebroadcast1To16Uint8x16(<16 x i8> %xbroadcast1To16Uint8x16
// LLVM-ARM64-DAG: extractelement <16 x i8> %xbroadcast1To16Uint8x16, i32 0
// LLVM-ARM64-DAG: shufflevector <16 x i8> {{%[^,]+}}, <16 x i8> zeroinitializer, <16 x i32> zeroinitializer
// LLVM-ARM64-DAG: define {{.*}} <2 x double> @codegen.llvmShufflebroadcast1To2Float64x2(<2 x double> %xbroadcast1To2Float64x2
// LLVM-ARM64-DAG: extractelement <2 x double> %xbroadcast1To2Float64x2, i32 0
// LLVM-ARM64-DAG: shufflevector <2 x double> {{%[^,]+}}, <2 x double> zeroinitializer, <2 x i32> zeroinitializer
// LLVM-ARM64-DAG: define {{.*}} <2 x i64> @codegen.llvmShufflebroadcast1To2Int64x2(<2 x i64> %xbroadcast1To2Int64x2
// LLVM-ARM64-DAG: extractelement <2 x i64> %xbroadcast1To2Int64x2, i32 0
// LLVM-ARM64-DAG: shufflevector <2 x i64> {{%[^,]+}}, <2 x i64> zeroinitializer, <2 x i32> zeroinitializer
// LLVM-ARM64-DAG: define {{.*}} <2 x i64> @codegen.llvmShufflebroadcast1To2Uint64x2(<2 x i64> %xbroadcast1To2Uint64x2
// LLVM-ARM64-DAG: extractelement <2 x i64> %xbroadcast1To2Uint64x2, i32 0
// LLVM-ARM64-DAG: shufflevector <2 x i64> {{%[^,]+}}, <2 x i64> zeroinitializer, <2 x i32> zeroinitializer
// LLVM-ARM64-DAG: define {{.*}} <4 x float> @codegen.llvmShufflebroadcast1To4Float32x4(<4 x float> %xbroadcast1To4Float32x4
// LLVM-ARM64-DAG: extractelement <4 x float> %xbroadcast1To4Float32x4, i32 0
// LLVM-ARM64-DAG: shufflevector <4 x float> {{%[^,]+}}, <4 x float> zeroinitializer, <4 x i32> zeroinitializer
// LLVM-ARM64-DAG: define {{.*}} <4 x i32> @codegen.llvmShufflebroadcast1To4Int32x4(<4 x i32> %xbroadcast1To4Int32x4
// LLVM-ARM64-DAG: extractelement <4 x i32> %xbroadcast1To4Int32x4, i32 0
// LLVM-ARM64-DAG: shufflevector <4 x i32> {{%[^,]+}}, <4 x i32> zeroinitializer, <4 x i32> zeroinitializer
// LLVM-ARM64-DAG: define {{.*}} <4 x i32> @codegen.llvmShufflebroadcast1To4Uint32x4(<4 x i32> %xbroadcast1To4Uint32x4
// LLVM-ARM64-DAG: extractelement <4 x i32> %xbroadcast1To4Uint32x4, i32 0
// LLVM-ARM64-DAG: shufflevector <4 x i32> {{%[^,]+}}, <4 x i32> zeroinitializer, <4 x i32> zeroinitializer
// LLVM-ARM64-DAG: define {{.*}} <8 x i16> @codegen.llvmShufflebroadcast1To8Int16x8(<8 x i16> %xbroadcast1To8Int16x8
// LLVM-ARM64-DAG: extractelement <8 x i16> %xbroadcast1To8Int16x8, i32 0
// LLVM-ARM64-DAG: shufflevector <8 x i16> {{%[^,]+}}, <8 x i16> zeroinitializer, <8 x i32> zeroinitializer
// LLVM-ARM64-DAG: define {{.*}} <8 x i16> @codegen.llvmShufflebroadcast1To8Uint16x8(<8 x i16> %xbroadcast1To8Uint16x8
// LLVM-ARM64-DAG: extractelement <8 x i16> %xbroadcast1To8Uint16x8, i32 0
// LLVM-ARM64-DAG: shufflevector <8 x i16> {{%[^,]+}}, <8 x i16> zeroinitializer, <8 x i32> zeroinitializer
// LLVM-ARM64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleConcatEvenInt16x8(<8 x i16> %xConcatEvenInt16x8
// LLVM-ARM64-DAG: shufflevector <8 x i16> %xConcatEvenInt16x8, <8 x i16> %yConcatEvenInt16x8, <8 x i32> <i32 0, i32 2, i32 4, i32 6, i32 8, i32 10, i32 12, i32 14>
// LLVM-ARM64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleConcatEvenInt32x4(<4 x i32> %xConcatEvenInt32x4
// LLVM-ARM64-DAG: shufflevector <4 x i32> %xConcatEvenInt32x4, <4 x i32> %yConcatEvenInt32x4, <4 x i32> <i32 0, i32 2, i32 4, i32 6>
// LLVM-ARM64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleConcatEvenInt64x2(<2 x i64> %xConcatEvenInt64x2
// LLVM-ARM64-DAG: shufflevector <2 x i64> %xConcatEvenInt64x2, <2 x i64> %yConcatEvenInt64x2, <2 x i32> <i32 0, i32 2>
// LLVM-ARM64-DAG: define {{.*}} <16 x i8> @codegen.llvmShuffleConcatEvenInt8x16(<16 x i8> %xConcatEvenInt8x16
// LLVM-ARM64-DAG: shufflevector <16 x i8> %xConcatEvenInt8x16, <16 x i8> %yConcatEvenInt8x16, <16 x i32> <i32 0, i32 2, i32 4, i32 6, i32 8, i32 10, i32 12, i32 14, i32 16, i32 18, i32 20, i32 22, i32 24, i32 26, i32 28, i32 30>
// LLVM-ARM64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleConcatEvenUint16x8(<8 x i16> %xConcatEvenUint16x8
// LLVM-ARM64-DAG: shufflevector <8 x i16> %xConcatEvenUint16x8, <8 x i16> %yConcatEvenUint16x8, <8 x i32> <i32 0, i32 2, i32 4, i32 6, i32 8, i32 10, i32 12, i32 14>
// LLVM-ARM64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleConcatEvenUint32x4(<4 x i32> %xConcatEvenUint32x4
// LLVM-ARM64-DAG: shufflevector <4 x i32> %xConcatEvenUint32x4, <4 x i32> %yConcatEvenUint32x4, <4 x i32> <i32 0, i32 2, i32 4, i32 6>
// LLVM-ARM64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleConcatEvenUint64x2(<2 x i64> %xConcatEvenUint64x2
// LLVM-ARM64-DAG: shufflevector <2 x i64> %xConcatEvenUint64x2, <2 x i64> %yConcatEvenUint64x2, <2 x i32> <i32 0, i32 2>
// LLVM-ARM64-DAG: define {{.*}} <16 x i8> @codegen.llvmShuffleConcatEvenUint8x16(<16 x i8> %xConcatEvenUint8x16
// LLVM-ARM64-DAG: shufflevector <16 x i8> %xConcatEvenUint8x16, <16 x i8> %yConcatEvenUint8x16, <16 x i32> <i32 0, i32 2, i32 4, i32 6, i32 8, i32 10, i32 12, i32 14, i32 16, i32 18, i32 20, i32 22, i32 24, i32 26, i32 28, i32 30>
// LLVM-ARM64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleConcatOddInt16x8(<8 x i16> %xConcatOddInt16x8
// LLVM-ARM64-DAG: shufflevector <8 x i16> %xConcatOddInt16x8, <8 x i16> %yConcatOddInt16x8, <8 x i32> <i32 1, i32 3, i32 5, i32 7, i32 9, i32 11, i32 13, i32 15>
// LLVM-ARM64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleConcatOddInt32x4(<4 x i32> %xConcatOddInt32x4
// LLVM-ARM64-DAG: shufflevector <4 x i32> %xConcatOddInt32x4, <4 x i32> %yConcatOddInt32x4, <4 x i32> <i32 1, i32 3, i32 5, i32 7>
// LLVM-ARM64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleConcatOddInt64x2(<2 x i64> %xConcatOddInt64x2
// LLVM-ARM64-DAG: shufflevector <2 x i64> %xConcatOddInt64x2, <2 x i64> %yConcatOddInt64x2, <2 x i32> <i32 1, i32 3>
// LLVM-ARM64-DAG: define {{.*}} <16 x i8> @codegen.llvmShuffleConcatOddInt8x16(<16 x i8> %xConcatOddInt8x16
// LLVM-ARM64-DAG: shufflevector <16 x i8> %xConcatOddInt8x16, <16 x i8> %yConcatOddInt8x16, <16 x i32> <i32 1, i32 3, i32 5, i32 7, i32 9, i32 11, i32 13, i32 15, i32 17, i32 19, i32 21, i32 23, i32 25, i32 27, i32 29, i32 31>
// LLVM-ARM64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleConcatOddUint16x8(<8 x i16> %xConcatOddUint16x8
// LLVM-ARM64-DAG: shufflevector <8 x i16> %xConcatOddUint16x8, <8 x i16> %yConcatOddUint16x8, <8 x i32> <i32 1, i32 3, i32 5, i32 7, i32 9, i32 11, i32 13, i32 15>
// LLVM-ARM64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleConcatOddUint32x4(<4 x i32> %xConcatOddUint32x4
// LLVM-ARM64-DAG: shufflevector <4 x i32> %xConcatOddUint32x4, <4 x i32> %yConcatOddUint32x4, <4 x i32> <i32 1, i32 3, i32 5, i32 7>
// LLVM-ARM64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleConcatOddUint64x2(<2 x i64> %xConcatOddUint64x2
// LLVM-ARM64-DAG: shufflevector <2 x i64> %xConcatOddUint64x2, <2 x i64> %yConcatOddUint64x2, <2 x i32> <i32 1, i32 3>
// LLVM-ARM64-DAG: define {{.*}} <16 x i8> @codegen.llvmShuffleConcatOddUint8x16(<16 x i8> %xConcatOddUint8x16
// LLVM-ARM64-DAG: shufflevector <16 x i8> %xConcatOddUint8x16, <16 x i8> %yConcatOddUint8x16, <16 x i32> <i32 1, i32 3, i32 5, i32 7, i32 9, i32 11, i32 13, i32 15, i32 17, i32 19, i32 21, i32 23, i32 25, i32 27, i32 29, i32 31>
// LLVM-ARM64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleInterleaveEvenInt16x8(<8 x i16> %xInterleaveEvenInt16x8
// LLVM-ARM64-DAG: shufflevector <8 x i16> %xInterleaveEvenInt16x8, <8 x i16> %yInterleaveEvenInt16x8, <8 x i32> <i32 0, i32 8, i32 2, i32 10, i32 4, i32 12, i32 6, i32 14>
// LLVM-ARM64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleInterleaveEvenInt32x4(<4 x i32> %xInterleaveEvenInt32x4
// LLVM-ARM64-DAG: shufflevector <4 x i32> %xInterleaveEvenInt32x4, <4 x i32> %yInterleaveEvenInt32x4, <4 x i32> <i32 0, i32 4, i32 2, i32 6>
// LLVM-ARM64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleInterleaveEvenInt64x2(<2 x i64> %xInterleaveEvenInt64x2
// LLVM-ARM64-DAG: shufflevector <2 x i64> %xInterleaveEvenInt64x2, <2 x i64> %yInterleaveEvenInt64x2, <2 x i32> <i32 0, i32 2>
// LLVM-ARM64-DAG: define {{.*}} <16 x i8> @codegen.llvmShuffleInterleaveEvenInt8x16(<16 x i8> %xInterleaveEvenInt8x16
// LLVM-ARM64-DAG: shufflevector <16 x i8> %xInterleaveEvenInt8x16, <16 x i8> %yInterleaveEvenInt8x16, <16 x i32> <i32 0, i32 16, i32 2, i32 18, i32 4, i32 20, i32 6, i32 22, i32 8, i32 24, i32 10, i32 26, i32 12, i32 28, i32 14, i32 30>
// LLVM-ARM64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleInterleaveEvenUint16x8(<8 x i16> %xInterleaveEvenUint16x8
// LLVM-ARM64-DAG: shufflevector <8 x i16> %xInterleaveEvenUint16x8, <8 x i16> %yInterleaveEvenUint16x8, <8 x i32> <i32 0, i32 8, i32 2, i32 10, i32 4, i32 12, i32 6, i32 14>
// LLVM-ARM64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleInterleaveEvenUint32x4(<4 x i32> %xInterleaveEvenUint32x4
// LLVM-ARM64-DAG: shufflevector <4 x i32> %xInterleaveEvenUint32x4, <4 x i32> %yInterleaveEvenUint32x4, <4 x i32> <i32 0, i32 4, i32 2, i32 6>
// LLVM-ARM64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleInterleaveEvenUint64x2(<2 x i64> %xInterleaveEvenUint64x2
// LLVM-ARM64-DAG: shufflevector <2 x i64> %xInterleaveEvenUint64x2, <2 x i64> %yInterleaveEvenUint64x2, <2 x i32> <i32 0, i32 2>
// LLVM-ARM64-DAG: define {{.*}} <16 x i8> @codegen.llvmShuffleInterleaveEvenUint8x16(<16 x i8> %xInterleaveEvenUint8x16
// LLVM-ARM64-DAG: shufflevector <16 x i8> %xInterleaveEvenUint8x16, <16 x i8> %yInterleaveEvenUint8x16, <16 x i32> <i32 0, i32 16, i32 2, i32 18, i32 4, i32 20, i32 6, i32 22, i32 8, i32 24, i32 10, i32 26, i32 12, i32 28, i32 14, i32 30>
// LLVM-ARM64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleInterleaveHiInt16x8(<8 x i16> %xInterleaveHiInt16x8
// LLVM-ARM64-DAG: shufflevector <8 x i16> %xInterleaveHiInt16x8, <8 x i16> %yInterleaveHiInt16x8, <8 x i32> <i32 4, i32 12, i32 5, i32 13, i32 6, i32 14, i32 7, i32 15>
// LLVM-ARM64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleInterleaveHiInt32x4(<4 x i32> %xInterleaveHiInt32x4
// LLVM-ARM64-DAG: shufflevector <4 x i32> %xInterleaveHiInt32x4, <4 x i32> %yInterleaveHiInt32x4, <4 x i32> <i32 2, i32 6, i32 3, i32 7>
// LLVM-ARM64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleInterleaveHiInt64x2(<2 x i64> %xInterleaveHiInt64x2
// LLVM-ARM64-DAG: shufflevector <2 x i64> %xInterleaveHiInt64x2, <2 x i64> %yInterleaveHiInt64x2, <2 x i32> <i32 1, i32 3>
// LLVM-ARM64-DAG: define {{.*}} <16 x i8> @codegen.llvmShuffleInterleaveHiInt8x16(<16 x i8> %xInterleaveHiInt8x16
// LLVM-ARM64-DAG: shufflevector <16 x i8> %xInterleaveHiInt8x16, <16 x i8> %yInterleaveHiInt8x16, <16 x i32> <i32 8, i32 24, i32 9, i32 25, i32 10, i32 26, i32 11, i32 27, i32 12, i32 28, i32 13, i32 29, i32 14, i32 30, i32 15, i32 31>
// LLVM-ARM64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleInterleaveHiUint16x8(<8 x i16> %xInterleaveHiUint16x8
// LLVM-ARM64-DAG: shufflevector <8 x i16> %xInterleaveHiUint16x8, <8 x i16> %yInterleaveHiUint16x8, <8 x i32> <i32 4, i32 12, i32 5, i32 13, i32 6, i32 14, i32 7, i32 15>
// LLVM-ARM64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleInterleaveHiUint32x4(<4 x i32> %xInterleaveHiUint32x4
// LLVM-ARM64-DAG: shufflevector <4 x i32> %xInterleaveHiUint32x4, <4 x i32> %yInterleaveHiUint32x4, <4 x i32> <i32 2, i32 6, i32 3, i32 7>
// LLVM-ARM64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleInterleaveHiUint64x2(<2 x i64> %xInterleaveHiUint64x2
// LLVM-ARM64-DAG: shufflevector <2 x i64> %xInterleaveHiUint64x2, <2 x i64> %yInterleaveHiUint64x2, <2 x i32> <i32 1, i32 3>
// LLVM-ARM64-DAG: define {{.*}} <16 x i8> @codegen.llvmShuffleInterleaveHiUint8x16(<16 x i8> %xInterleaveHiUint8x16
// LLVM-ARM64-DAG: shufflevector <16 x i8> %xInterleaveHiUint8x16, <16 x i8> %yInterleaveHiUint8x16, <16 x i32> <i32 8, i32 24, i32 9, i32 25, i32 10, i32 26, i32 11, i32 27, i32 12, i32 28, i32 13, i32 29, i32 14, i32 30, i32 15, i32 31>
// LLVM-ARM64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleInterleaveLoInt16x8(<8 x i16> %xInterleaveLoInt16x8
// LLVM-ARM64-DAG: shufflevector <8 x i16> %xInterleaveLoInt16x8, <8 x i16> %yInterleaveLoInt16x8, <8 x i32> <i32 0, i32 8, i32 1, i32 9, i32 2, i32 10, i32 3, i32 11>
// LLVM-ARM64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleInterleaveLoInt32x4(<4 x i32> %xInterleaveLoInt32x4
// LLVM-ARM64-DAG: shufflevector <4 x i32> %xInterleaveLoInt32x4, <4 x i32> %yInterleaveLoInt32x4, <4 x i32> <i32 0, i32 4, i32 1, i32 5>
// LLVM-ARM64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleInterleaveLoInt64x2(<2 x i64> %xInterleaveLoInt64x2
// LLVM-ARM64-DAG: shufflevector <2 x i64> %xInterleaveLoInt64x2, <2 x i64> %yInterleaveLoInt64x2, <2 x i32> <i32 0, i32 2>
// LLVM-ARM64-DAG: define {{.*}} <16 x i8> @codegen.llvmShuffleInterleaveLoInt8x16(<16 x i8> %xInterleaveLoInt8x16
// LLVM-ARM64-DAG: shufflevector <16 x i8> %xInterleaveLoInt8x16, <16 x i8> %yInterleaveLoInt8x16, <16 x i32> <i32 0, i32 16, i32 1, i32 17, i32 2, i32 18, i32 3, i32 19, i32 4, i32 20, i32 5, i32 21, i32 6, i32 22, i32 7, i32 23>
// LLVM-ARM64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleInterleaveLoUint16x8(<8 x i16> %xInterleaveLoUint16x8
// LLVM-ARM64-DAG: shufflevector <8 x i16> %xInterleaveLoUint16x8, <8 x i16> %yInterleaveLoUint16x8, <8 x i32> <i32 0, i32 8, i32 1, i32 9, i32 2, i32 10, i32 3, i32 11>
// LLVM-ARM64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleInterleaveLoUint32x4(<4 x i32> %xInterleaveLoUint32x4
// LLVM-ARM64-DAG: shufflevector <4 x i32> %xInterleaveLoUint32x4, <4 x i32> %yInterleaveLoUint32x4, <4 x i32> <i32 0, i32 4, i32 1, i32 5>
// LLVM-ARM64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleInterleaveLoUint64x2(<2 x i64> %xInterleaveLoUint64x2
// LLVM-ARM64-DAG: shufflevector <2 x i64> %xInterleaveLoUint64x2, <2 x i64> %yInterleaveLoUint64x2, <2 x i32> <i32 0, i32 2>
// LLVM-ARM64-DAG: define {{.*}} <16 x i8> @codegen.llvmShuffleInterleaveLoUint8x16(<16 x i8> %xInterleaveLoUint8x16
// LLVM-ARM64-DAG: shufflevector <16 x i8> %xInterleaveLoUint8x16, <16 x i8> %yInterleaveLoUint8x16, <16 x i32> <i32 0, i32 16, i32 1, i32 17, i32 2, i32 18, i32 3, i32 19, i32 4, i32 20, i32 5, i32 21, i32 6, i32 22, i32 7, i32 23>
// LLVM-ARM64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleInterleaveOddInt16x8(<8 x i16> %xInterleaveOddInt16x8
// LLVM-ARM64-DAG: shufflevector <8 x i16> %xInterleaveOddInt16x8, <8 x i16> %yInterleaveOddInt16x8, <8 x i32> <i32 1, i32 9, i32 3, i32 11, i32 5, i32 13, i32 7, i32 15>
// LLVM-ARM64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleInterleaveOddInt32x4(<4 x i32> %xInterleaveOddInt32x4
// LLVM-ARM64-DAG: shufflevector <4 x i32> %xInterleaveOddInt32x4, <4 x i32> %yInterleaveOddInt32x4, <4 x i32> <i32 1, i32 5, i32 3, i32 7>
// LLVM-ARM64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleInterleaveOddInt64x2(<2 x i64> %xInterleaveOddInt64x2
// LLVM-ARM64-DAG: shufflevector <2 x i64> %xInterleaveOddInt64x2, <2 x i64> %yInterleaveOddInt64x2, <2 x i32> <i32 1, i32 3>
// LLVM-ARM64-DAG: define {{.*}} <16 x i8> @codegen.llvmShuffleInterleaveOddInt8x16(<16 x i8> %xInterleaveOddInt8x16
// LLVM-ARM64-DAG: shufflevector <16 x i8> %xInterleaveOddInt8x16, <16 x i8> %yInterleaveOddInt8x16, <16 x i32> <i32 1, i32 17, i32 3, i32 19, i32 5, i32 21, i32 7, i32 23, i32 9, i32 25, i32 11, i32 27, i32 13, i32 29, i32 15, i32 31>
// LLVM-ARM64-DAG: define {{.*}} <8 x i16> @codegen.llvmShuffleInterleaveOddUint16x8(<8 x i16> %xInterleaveOddUint16x8
// LLVM-ARM64-DAG: shufflevector <8 x i16> %xInterleaveOddUint16x8, <8 x i16> %yInterleaveOddUint16x8, <8 x i32> <i32 1, i32 9, i32 3, i32 11, i32 5, i32 13, i32 7, i32 15>
// LLVM-ARM64-DAG: define {{.*}} <4 x i32> @codegen.llvmShuffleInterleaveOddUint32x4(<4 x i32> %xInterleaveOddUint32x4
// LLVM-ARM64-DAG: shufflevector <4 x i32> %xInterleaveOddUint32x4, <4 x i32> %yInterleaveOddUint32x4, <4 x i32> <i32 1, i32 5, i32 3, i32 7>
// LLVM-ARM64-DAG: define {{.*}} <2 x i64> @codegen.llvmShuffleInterleaveOddUint64x2(<2 x i64> %xInterleaveOddUint64x2
// LLVM-ARM64-DAG: shufflevector <2 x i64> %xInterleaveOddUint64x2, <2 x i64> %yInterleaveOddUint64x2, <2 x i32> <i32 1, i32 3>
// LLVM-ARM64-DAG: define {{.*}} <16 x i8> @codegen.llvmShuffleInterleaveOddUint8x16(<16 x i8> %xInterleaveOddUint8x16
// LLVM-ARM64-DAG: shufflevector <16 x i8> %xInterleaveOddUint8x16, <16 x i8> %yInterleaveOddUint8x16, <16 x i32> <i32 1, i32 17, i32 3, i32 19, i32 5, i32 21, i32 7, i32 23, i32 9, i32 25, i32 11, i32 27, i32 13, i32 29, i32 15, i32 31>

//go:noinline
func llvmShufflebroadcast1To16Int8x16(xbroadcast1To16Int8x16 archsimd.Int8x16) archsimd.Int8x16 {
	return archsimd.BroadcastInt8x16(xbroadcast1To16Int8x16.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To16Uint8x16(xbroadcast1To16Uint8x16 archsimd.Uint8x16) archsimd.Uint8x16 {
	return archsimd.BroadcastUint8x16(xbroadcast1To16Uint8x16.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To2Float64x2(xbroadcast1To2Float64x2 archsimd.Float64x2) archsimd.Float64x2 {
	return archsimd.BroadcastFloat64x2(xbroadcast1To2Float64x2.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To2Int64x2(xbroadcast1To2Int64x2 archsimd.Int64x2) archsimd.Int64x2 {
	return archsimd.BroadcastInt64x2(xbroadcast1To2Int64x2.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To2Uint64x2(xbroadcast1To2Uint64x2 archsimd.Uint64x2) archsimd.Uint64x2 {
	return archsimd.BroadcastUint64x2(xbroadcast1To2Uint64x2.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To4Float32x4(xbroadcast1To4Float32x4 archsimd.Float32x4) archsimd.Float32x4 {
	return archsimd.BroadcastFloat32x4(xbroadcast1To4Float32x4.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To4Int32x4(xbroadcast1To4Int32x4 archsimd.Int32x4) archsimd.Int32x4 {
	return archsimd.BroadcastInt32x4(xbroadcast1To4Int32x4.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To4Uint32x4(xbroadcast1To4Uint32x4 archsimd.Uint32x4) archsimd.Uint32x4 {
	return archsimd.BroadcastUint32x4(xbroadcast1To4Uint32x4.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To8Int16x8(xbroadcast1To8Int16x8 archsimd.Int16x8) archsimd.Int16x8 {
	return archsimd.BroadcastInt16x8(xbroadcast1To8Int16x8.GetElem(0))
}

//go:noinline
func llvmShufflebroadcast1To8Uint16x8(xbroadcast1To8Uint16x8 archsimd.Uint16x8) archsimd.Uint16x8 {
	return archsimd.BroadcastUint16x8(xbroadcast1To8Uint16x8.GetElem(0))
}

//go:noinline
func llvmShuffleConcatEvenInt16x8(xConcatEvenInt16x8 archsimd.Int16x8, yConcatEvenInt16x8 archsimd.Int16x8) archsimd.Int16x8 {
	return xConcatEvenInt16x8.ConcatEven(yConcatEvenInt16x8)
}

//go:noinline
func llvmShuffleConcatEvenInt32x4(xConcatEvenInt32x4 archsimd.Int32x4, yConcatEvenInt32x4 archsimd.Int32x4) archsimd.Int32x4 {
	return xConcatEvenInt32x4.ConcatEven(yConcatEvenInt32x4)
}

//go:noinline
func llvmShuffleConcatEvenInt64x2(xConcatEvenInt64x2 archsimd.Int64x2, yConcatEvenInt64x2 archsimd.Int64x2) archsimd.Int64x2 {
	return xConcatEvenInt64x2.ConcatEven(yConcatEvenInt64x2)
}

//go:noinline
func llvmShuffleConcatEvenInt8x16(xConcatEvenInt8x16 archsimd.Int8x16, yConcatEvenInt8x16 archsimd.Int8x16) archsimd.Int8x16 {
	return xConcatEvenInt8x16.ConcatEven(yConcatEvenInt8x16)
}

//go:noinline
func llvmShuffleConcatEvenUint16x8(xConcatEvenUint16x8 archsimd.Uint16x8, yConcatEvenUint16x8 archsimd.Uint16x8) archsimd.Uint16x8 {
	return xConcatEvenUint16x8.ConcatEven(yConcatEvenUint16x8)
}

//go:noinline
func llvmShuffleConcatEvenUint32x4(xConcatEvenUint32x4 archsimd.Uint32x4, yConcatEvenUint32x4 archsimd.Uint32x4) archsimd.Uint32x4 {
	return xConcatEvenUint32x4.ConcatEven(yConcatEvenUint32x4)
}

//go:noinline
func llvmShuffleConcatEvenUint64x2(xConcatEvenUint64x2 archsimd.Uint64x2, yConcatEvenUint64x2 archsimd.Uint64x2) archsimd.Uint64x2 {
	return xConcatEvenUint64x2.ConcatEven(yConcatEvenUint64x2)
}

//go:noinline
func llvmShuffleConcatEvenUint8x16(xConcatEvenUint8x16 archsimd.Uint8x16, yConcatEvenUint8x16 archsimd.Uint8x16) archsimd.Uint8x16 {
	return xConcatEvenUint8x16.ConcatEven(yConcatEvenUint8x16)
}

//go:noinline
func llvmShuffleConcatOddInt16x8(xConcatOddInt16x8 archsimd.Int16x8, yConcatOddInt16x8 archsimd.Int16x8) archsimd.Int16x8 {
	return xConcatOddInt16x8.ConcatOdd(yConcatOddInt16x8)
}

//go:noinline
func llvmShuffleConcatOddInt32x4(xConcatOddInt32x4 archsimd.Int32x4, yConcatOddInt32x4 archsimd.Int32x4) archsimd.Int32x4 {
	return xConcatOddInt32x4.ConcatOdd(yConcatOddInt32x4)
}

//go:noinline
func llvmShuffleConcatOddInt64x2(xConcatOddInt64x2 archsimd.Int64x2, yConcatOddInt64x2 archsimd.Int64x2) archsimd.Int64x2 {
	return xConcatOddInt64x2.ConcatOdd(yConcatOddInt64x2)
}

//go:noinline
func llvmShuffleConcatOddInt8x16(xConcatOddInt8x16 archsimd.Int8x16, yConcatOddInt8x16 archsimd.Int8x16) archsimd.Int8x16 {
	return xConcatOddInt8x16.ConcatOdd(yConcatOddInt8x16)
}

//go:noinline
func llvmShuffleConcatOddUint16x8(xConcatOddUint16x8 archsimd.Uint16x8, yConcatOddUint16x8 archsimd.Uint16x8) archsimd.Uint16x8 {
	return xConcatOddUint16x8.ConcatOdd(yConcatOddUint16x8)
}

//go:noinline
func llvmShuffleConcatOddUint32x4(xConcatOddUint32x4 archsimd.Uint32x4, yConcatOddUint32x4 archsimd.Uint32x4) archsimd.Uint32x4 {
	return xConcatOddUint32x4.ConcatOdd(yConcatOddUint32x4)
}

//go:noinline
func llvmShuffleConcatOddUint64x2(xConcatOddUint64x2 archsimd.Uint64x2, yConcatOddUint64x2 archsimd.Uint64x2) archsimd.Uint64x2 {
	return xConcatOddUint64x2.ConcatOdd(yConcatOddUint64x2)
}

//go:noinline
func llvmShuffleConcatOddUint8x16(xConcatOddUint8x16 archsimd.Uint8x16, yConcatOddUint8x16 archsimd.Uint8x16) archsimd.Uint8x16 {
	return xConcatOddUint8x16.ConcatOdd(yConcatOddUint8x16)
}

//go:noinline
func llvmShuffleInterleaveEvenInt16x8(xInterleaveEvenInt16x8 archsimd.Int16x8, yInterleaveEvenInt16x8 archsimd.Int16x8) archsimd.Int16x8 {
	return xInterleaveEvenInt16x8.InterleaveEven(yInterleaveEvenInt16x8)
}

//go:noinline
func llvmShuffleInterleaveEvenInt32x4(xInterleaveEvenInt32x4 archsimd.Int32x4, yInterleaveEvenInt32x4 archsimd.Int32x4) archsimd.Int32x4 {
	return xInterleaveEvenInt32x4.InterleaveEven(yInterleaveEvenInt32x4)
}

//go:noinline
func llvmShuffleInterleaveEvenInt64x2(xInterleaveEvenInt64x2 archsimd.Int64x2, yInterleaveEvenInt64x2 archsimd.Int64x2) archsimd.Int64x2 {
	return xInterleaveEvenInt64x2.InterleaveEven(yInterleaveEvenInt64x2)
}

//go:noinline
func llvmShuffleInterleaveEvenInt8x16(xInterleaveEvenInt8x16 archsimd.Int8x16, yInterleaveEvenInt8x16 archsimd.Int8x16) archsimd.Int8x16 {
	return xInterleaveEvenInt8x16.InterleaveEven(yInterleaveEvenInt8x16)
}

//go:noinline
func llvmShuffleInterleaveEvenUint16x8(xInterleaveEvenUint16x8 archsimd.Uint16x8, yInterleaveEvenUint16x8 archsimd.Uint16x8) archsimd.Uint16x8 {
	return xInterleaveEvenUint16x8.InterleaveEven(yInterleaveEvenUint16x8)
}

//go:noinline
func llvmShuffleInterleaveEvenUint32x4(xInterleaveEvenUint32x4 archsimd.Uint32x4, yInterleaveEvenUint32x4 archsimd.Uint32x4) archsimd.Uint32x4 {
	return xInterleaveEvenUint32x4.InterleaveEven(yInterleaveEvenUint32x4)
}

//go:noinline
func llvmShuffleInterleaveEvenUint64x2(xInterleaveEvenUint64x2 archsimd.Uint64x2, yInterleaveEvenUint64x2 archsimd.Uint64x2) archsimd.Uint64x2 {
	return xInterleaveEvenUint64x2.InterleaveEven(yInterleaveEvenUint64x2)
}

//go:noinline
func llvmShuffleInterleaveEvenUint8x16(xInterleaveEvenUint8x16 archsimd.Uint8x16, yInterleaveEvenUint8x16 archsimd.Uint8x16) archsimd.Uint8x16 {
	return xInterleaveEvenUint8x16.InterleaveEven(yInterleaveEvenUint8x16)
}

//go:noinline
func llvmShuffleInterleaveHiInt16x8(xInterleaveHiInt16x8 archsimd.Int16x8, yInterleaveHiInt16x8 archsimd.Int16x8) archsimd.Int16x8 {
	return xInterleaveHiInt16x8.InterleaveHi(yInterleaveHiInt16x8)
}

//go:noinline
func llvmShuffleInterleaveHiInt32x4(xInterleaveHiInt32x4 archsimd.Int32x4, yInterleaveHiInt32x4 archsimd.Int32x4) archsimd.Int32x4 {
	return xInterleaveHiInt32x4.InterleaveHi(yInterleaveHiInt32x4)
}

//go:noinline
func llvmShuffleInterleaveHiInt64x2(xInterleaveHiInt64x2 archsimd.Int64x2, yInterleaveHiInt64x2 archsimd.Int64x2) archsimd.Int64x2 {
	return xInterleaveHiInt64x2.InterleaveHi(yInterleaveHiInt64x2)
}

//go:noinline
func llvmShuffleInterleaveHiInt8x16(xInterleaveHiInt8x16 archsimd.Int8x16, yInterleaveHiInt8x16 archsimd.Int8x16) archsimd.Int8x16 {
	return xInterleaveHiInt8x16.InterleaveHi(yInterleaveHiInt8x16)
}

//go:noinline
func llvmShuffleInterleaveHiUint16x8(xInterleaveHiUint16x8 archsimd.Uint16x8, yInterleaveHiUint16x8 archsimd.Uint16x8) archsimd.Uint16x8 {
	return xInterleaveHiUint16x8.InterleaveHi(yInterleaveHiUint16x8)
}

//go:noinline
func llvmShuffleInterleaveHiUint32x4(xInterleaveHiUint32x4 archsimd.Uint32x4, yInterleaveHiUint32x4 archsimd.Uint32x4) archsimd.Uint32x4 {
	return xInterleaveHiUint32x4.InterleaveHi(yInterleaveHiUint32x4)
}

//go:noinline
func llvmShuffleInterleaveHiUint64x2(xInterleaveHiUint64x2 archsimd.Uint64x2, yInterleaveHiUint64x2 archsimd.Uint64x2) archsimd.Uint64x2 {
	return xInterleaveHiUint64x2.InterleaveHi(yInterleaveHiUint64x2)
}

//go:noinline
func llvmShuffleInterleaveHiUint8x16(xInterleaveHiUint8x16 archsimd.Uint8x16, yInterleaveHiUint8x16 archsimd.Uint8x16) archsimd.Uint8x16 {
	return xInterleaveHiUint8x16.InterleaveHi(yInterleaveHiUint8x16)
}

//go:noinline
func llvmShuffleInterleaveLoInt16x8(xInterleaveLoInt16x8 archsimd.Int16x8, yInterleaveLoInt16x8 archsimd.Int16x8) archsimd.Int16x8 {
	return xInterleaveLoInt16x8.InterleaveLo(yInterleaveLoInt16x8)
}

//go:noinline
func llvmShuffleInterleaveLoInt32x4(xInterleaveLoInt32x4 archsimd.Int32x4, yInterleaveLoInt32x4 archsimd.Int32x4) archsimd.Int32x4 {
	return xInterleaveLoInt32x4.InterleaveLo(yInterleaveLoInt32x4)
}

//go:noinline
func llvmShuffleInterleaveLoInt64x2(xInterleaveLoInt64x2 archsimd.Int64x2, yInterleaveLoInt64x2 archsimd.Int64x2) archsimd.Int64x2 {
	return xInterleaveLoInt64x2.InterleaveLo(yInterleaveLoInt64x2)
}

//go:noinline
func llvmShuffleInterleaveLoInt8x16(xInterleaveLoInt8x16 archsimd.Int8x16, yInterleaveLoInt8x16 archsimd.Int8x16) archsimd.Int8x16 {
	return xInterleaveLoInt8x16.InterleaveLo(yInterleaveLoInt8x16)
}

//go:noinline
func llvmShuffleInterleaveLoUint16x8(xInterleaveLoUint16x8 archsimd.Uint16x8, yInterleaveLoUint16x8 archsimd.Uint16x8) archsimd.Uint16x8 {
	return xInterleaveLoUint16x8.InterleaveLo(yInterleaveLoUint16x8)
}

//go:noinline
func llvmShuffleInterleaveLoUint32x4(xInterleaveLoUint32x4 archsimd.Uint32x4, yInterleaveLoUint32x4 archsimd.Uint32x4) archsimd.Uint32x4 {
	return xInterleaveLoUint32x4.InterleaveLo(yInterleaveLoUint32x4)
}

//go:noinline
func llvmShuffleInterleaveLoUint64x2(xInterleaveLoUint64x2 archsimd.Uint64x2, yInterleaveLoUint64x2 archsimd.Uint64x2) archsimd.Uint64x2 {
	return xInterleaveLoUint64x2.InterleaveLo(yInterleaveLoUint64x2)
}

//go:noinline
func llvmShuffleInterleaveLoUint8x16(xInterleaveLoUint8x16 archsimd.Uint8x16, yInterleaveLoUint8x16 archsimd.Uint8x16) archsimd.Uint8x16 {
	return xInterleaveLoUint8x16.InterleaveLo(yInterleaveLoUint8x16)
}

//go:noinline
func llvmShuffleInterleaveOddInt16x8(xInterleaveOddInt16x8 archsimd.Int16x8, yInterleaveOddInt16x8 archsimd.Int16x8) archsimd.Int16x8 {
	return xInterleaveOddInt16x8.InterleaveOdd(yInterleaveOddInt16x8)
}

//go:noinline
func llvmShuffleInterleaveOddInt32x4(xInterleaveOddInt32x4 archsimd.Int32x4, yInterleaveOddInt32x4 archsimd.Int32x4) archsimd.Int32x4 {
	return xInterleaveOddInt32x4.InterleaveOdd(yInterleaveOddInt32x4)
}

//go:noinline
func llvmShuffleInterleaveOddInt64x2(xInterleaveOddInt64x2 archsimd.Int64x2, yInterleaveOddInt64x2 archsimd.Int64x2) archsimd.Int64x2 {
	return xInterleaveOddInt64x2.InterleaveOdd(yInterleaveOddInt64x2)
}

//go:noinline
func llvmShuffleInterleaveOddInt8x16(xInterleaveOddInt8x16 archsimd.Int8x16, yInterleaveOddInt8x16 archsimd.Int8x16) archsimd.Int8x16 {
	return xInterleaveOddInt8x16.InterleaveOdd(yInterleaveOddInt8x16)
}

//go:noinline
func llvmShuffleInterleaveOddUint16x8(xInterleaveOddUint16x8 archsimd.Uint16x8, yInterleaveOddUint16x8 archsimd.Uint16x8) archsimd.Uint16x8 {
	return xInterleaveOddUint16x8.InterleaveOdd(yInterleaveOddUint16x8)
}

//go:noinline
func llvmShuffleInterleaveOddUint32x4(xInterleaveOddUint32x4 archsimd.Uint32x4, yInterleaveOddUint32x4 archsimd.Uint32x4) archsimd.Uint32x4 {
	return xInterleaveOddUint32x4.InterleaveOdd(yInterleaveOddUint32x4)
}

//go:noinline
func llvmShuffleInterleaveOddUint64x2(xInterleaveOddUint64x2 archsimd.Uint64x2, yInterleaveOddUint64x2 archsimd.Uint64x2) archsimd.Uint64x2 {
	return xInterleaveOddUint64x2.InterleaveOdd(yInterleaveOddUint64x2)
}

//go:noinline
func llvmShuffleInterleaveOddUint8x16(xInterleaveOddUint8x16 archsimd.Uint8x16, yInterleaveOddUint8x16 archsimd.Uint8x16) archsimd.Uint8x16 {
	return xInterleaveOddUint8x16.InterleaveOdd(yInterleaveOddUint8x16)
}
