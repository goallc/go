// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && amd64

package codegen

import "simd/archsimd"

// Equivalent signed/unsigned API types have the same LLVM lane type. Check
// every function and each distinct IR routing shape; exhaustive SSA and runtime
// tests separately cover every opcode and all control values.
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateConcatPermute128ScalarsFloat32x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateConcatPermute128ScalarsFloat64x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateConcatPermute128ScalarsInt8x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateConcatPermute128ScalarsInt16x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateConcatPermute128ScalarsInt32x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateConcatPermute128ScalarsInt64x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateConcatPermute128ScalarsUint8x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateConcatPermute128ScalarsUint16x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateConcatPermute128ScalarsUint32x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateConcatPermute128ScalarsUint64x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateConcatShiftBytesRightGroupedUint8x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateConcatShiftBytesRightGroupedUint8x64(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateConcatShiftBytesRightUint8x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantFloat32x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantFloat64x2(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantGroupedFloat32x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantGroupedFloat32x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantGroupedFloat64x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantGroupedFloat64x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantGroupedInt32x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantGroupedInt32x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantGroupedInt64x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantGroupedInt64x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantGroupedUint32x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantGroupedUint32x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantGroupedUint64x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantGroupedUint64x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantInt32x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantInt64x2(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantUint32x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediateconcatSelectedConstantUint64x2(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsGroupedInt32x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsGroupedInt32x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsGroupedUint32x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsGroupedUint32x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsHiGroupedInt16x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsHiGroupedInt16x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsHiGroupedUint16x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsHiGroupedUint16x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsHiInt16x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsHiUint16x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsInt32x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsLoGroupedInt16x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsLoGroupedInt16x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsLoGroupedUint16x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsLoGroupedUint16x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsLoInt16x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsLoUint16x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmImmediatepermuteScalarsUint32x4(
// LLVM-AMD64-DAG: shufflevector <8 x float> %{{[^, ]+}}, <8 x float> %{{[^, ]+}}, <8 x i32> <i32 12, i32 13, i32 14, i32 15, i32 0, i32 1, i32 2, i32 3>
// LLVM-AMD64-DAG: shufflevector <4 x double> %{{[^, ]+}}, <4 x double> %{{[^, ]+}}, <4 x i32> <i32 6, i32 7, i32 0, i32 1>
// LLVM-AMD64-DAG: shufflevector <32 x i8> %{{[^, ]+}}, <32 x i8> %{{[^, ]+}}, <32 x i32> <i32 48, i32 49, i32 50, i32 51, i32 52, i32 53, i32 54, i32 55, i32 56, i32 57, i32 58, i32 59, i32 60, i32 61, i32 62, i32 63, i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>
// LLVM-AMD64-DAG: shufflevector <16 x i16> %{{[^, ]+}}, <16 x i16> %{{[^, ]+}}, <16 x i32> <i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31, i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7>
// LLVM-AMD64-DAG: shufflevector <8 x i32> %{{[^, ]+}}, <8 x i32> %{{[^, ]+}}, <8 x i32> <i32 12, i32 13, i32 14, i32 15, i32 0, i32 1, i32 2, i32 3>
// LLVM-AMD64-DAG: shufflevector <4 x i64> %{{[^, ]+}}, <4 x i64> %{{[^, ]+}}, <4 x i32> <i32 6, i32 7, i32 0, i32 1>
// LLVM-AMD64-DAG: shufflevector <32 x i8> %{{[^, ]+}}, <32 x i8> %{{[^, ]+}}, <32 x i32> <i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 55, i32 56, i32 57, i32 58, i32 59, i32 60, i32 61, i32 62, i32 63, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22>
// LLVM-AMD64-DAG: shufflevector <64 x i8> %{{[^, ]+}}, <64 x i8> %{{[^, ]+}}, <64 x i32> <i32 71, i32 72, i32 73, i32 74, i32 75, i32 76, i32 77, i32 78, i32 79, i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 87, i32 88, i32 89, i32 90, i32 91, i32 92, i32 93, i32 94, i32 95, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 103, i32 104, i32 105, i32 106, i32 107, i32 108, i32 109, i32 110, i32 111, i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 119, i32 120, i32 121, i32 122, i32 123, i32 124, i32 125, i32 126, i32 127, i32 48, i32 49, i32 50, i32 51, i32 52, i32 53, i32 54>
// LLVM-AMD64-DAG: shufflevector <16 x i8> %{{[^, ]+}}, <16 x i8> %{{[^, ]+}}, <16 x i32> <i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31, i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6>
// LLVM-AMD64-DAG: shufflevector <4 x float> %{{[^, ]+}}, <4 x float> %{{[^, ]+}}, <4 x i32> <i32 2, i32 0, i32 5, i32 7>
// LLVM-AMD64-DAG: shufflevector <2 x double> %{{[^, ]+}}, <2 x double> %{{[^, ]+}}, <2 x i32> <i32 1, i32 2>
// LLVM-AMD64-DAG: shufflevector <8 x float> %{{[^, ]+}}, <8 x float> %{{[^, ]+}}, <8 x i32> <i32 2, i32 0, i32 9, i32 11, i32 6, i32 4, i32 13, i32 15>
// LLVM-AMD64-DAG: shufflevector <16 x float> %{{[^, ]+}}, <16 x float> %{{[^, ]+}}, <16 x i32> <i32 2, i32 0, i32 17, i32 19, i32 6, i32 4, i32 21, i32 23, i32 10, i32 8, i32 25, i32 27, i32 14, i32 12, i32 29, i32 31>
// LLVM-AMD64-DAG: shufflevector <4 x double> %{{[^, ]+}}, <4 x double> %{{[^, ]+}}, <4 x i32> <i32 1, i32 4, i32 3, i32 6>
// LLVM-AMD64-DAG: shufflevector <8 x double> %{{[^, ]+}}, <8 x double> %{{[^, ]+}}, <8 x i32> <i32 1, i32 8, i32 3, i32 10, i32 5, i32 12, i32 7, i32 14>
// LLVM-AMD64-DAG: shufflevector <8 x i32> %{{[^, ]+}}, <8 x i32> %{{[^, ]+}}, <8 x i32> <i32 2, i32 0, i32 9, i32 11, i32 6, i32 4, i32 13, i32 15>
// LLVM-AMD64-DAG: shufflevector <16 x i32> %{{[^, ]+}}, <16 x i32> %{{[^, ]+}}, <16 x i32> <i32 2, i32 0, i32 17, i32 19, i32 6, i32 4, i32 21, i32 23, i32 10, i32 8, i32 25, i32 27, i32 14, i32 12, i32 29, i32 31>
// LLVM-AMD64-DAG: shufflevector <4 x i64> %{{[^, ]+}}, <4 x i64> %{{[^, ]+}}, <4 x i32> <i32 1, i32 4, i32 3, i32 6>
// LLVM-AMD64-DAG: shufflevector <8 x i64> %{{[^, ]+}}, <8 x i64> %{{[^, ]+}}, <8 x i32> <i32 1, i32 8, i32 3, i32 10, i32 5, i32 12, i32 7, i32 14>
// LLVM-AMD64-DAG: shufflevector <4 x i32> %{{[^, ]+}}, <4 x i32> %{{[^, ]+}}, <4 x i32> <i32 2, i32 0, i32 5, i32 7>
// LLVM-AMD64-DAG: shufflevector <2 x i64> %{{[^, ]+}}, <2 x i64> %{{[^, ]+}}, <2 x i32> <i32 1, i32 2>
// LLVM-AMD64-DAG: shufflevector <8 x i32> %{{[^, ]+}}, <8 x i32> zeroinitializer, <8 x i32> <i32 3, i32 0, i32 2, i32 1, i32 7, i32 4, i32 6, i32 5>
// LLVM-AMD64-DAG: shufflevector <16 x i32> %{{[^, ]+}}, <16 x i32> zeroinitializer, <16 x i32> <i32 3, i32 0, i32 2, i32 1, i32 7, i32 4, i32 6, i32 5, i32 11, i32 8, i32 10, i32 9, i32 15, i32 12, i32 14, i32 13>
// LLVM-AMD64-DAG: shufflevector <16 x i16> %{{[^, ]+}}, <16 x i16> zeroinitializer, <16 x i32> <i32 0, i32 1, i32 2, i32 3, i32 7, i32 4, i32 6, i32 5, i32 8, i32 9, i32 10, i32 11, i32 15, i32 12, i32 14, i32 13>
// LLVM-AMD64-DAG: shufflevector <32 x i16> %{{[^, ]+}}, <32 x i16> zeroinitializer, <32 x i32> <i32 0, i32 1, i32 2, i32 3, i32 7, i32 4, i32 6, i32 5, i32 8, i32 9, i32 10, i32 11, i32 15, i32 12, i32 14, i32 13, i32 16, i32 17, i32 18, i32 19, i32 23, i32 20, i32 22, i32 21, i32 24, i32 25, i32 26, i32 27, i32 31, i32 28, i32 30, i32 29>
// LLVM-AMD64-DAG: shufflevector <8 x i16> %{{[^, ]+}}, <8 x i16> zeroinitializer, <8 x i32> <i32 0, i32 1, i32 2, i32 3, i32 7, i32 4, i32 6, i32 5>
// LLVM-AMD64-DAG: shufflevector <4 x i32> %{{[^, ]+}}, <4 x i32> zeroinitializer, <4 x i32> <i32 3, i32 0, i32 2, i32 1>
// LLVM-AMD64-DAG: shufflevector <16 x i16> %{{[^, ]+}}, <16 x i16> zeroinitializer, <16 x i32> <i32 3, i32 0, i32 2, i32 1, i32 4, i32 5, i32 6, i32 7, i32 11, i32 8, i32 10, i32 9, i32 12, i32 13, i32 14, i32 15>
// LLVM-AMD64-DAG: shufflevector <32 x i16> %{{[^, ]+}}, <32 x i16> zeroinitializer, <32 x i32> <i32 3, i32 0, i32 2, i32 1, i32 4, i32 5, i32 6, i32 7, i32 11, i32 8, i32 10, i32 9, i32 12, i32 13, i32 14, i32 15, i32 19, i32 16, i32 18, i32 17, i32 20, i32 21, i32 22, i32 23, i32 27, i32 24, i32 26, i32 25, i32 28, i32 29, i32 30, i32 31>
// LLVM-AMD64-DAG: shufflevector <8 x i16> %{{[^, ]+}}, <8 x i16> zeroinitializer, <8 x i32> <i32 3, i32 0, i32 2, i32 1, i32 4, i32 5, i32 6, i32 7>

// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmImmediatepermuteScalarsGroupedInt32x8<goallc.fmv.baseline>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmImmediatepermuteScalarsGroupedInt32x8<goallc.fmv.avx2>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmImmediatepermuteScalarsGroupedInt32x8<goallc.fmv.resolve>"
// LLVM-NM-AMD64-DAG: D codegen.llvmImmediatepermuteScalarsGroupedInt32x8.goallc.fmv.slot
// LLVM-ASM-AMD64-DAG: VSHUFPS $0x63, X1, X1, X0
// LLVM-ASM-AMD64-DAG: VSHUFPS $0x63, Y0, Y0, Y0
// LLVM-ASM-AMD64-DAG: VSHUFPS $0xd2, Z1, Z2, Z0
// LLVM-ASM-AMD64-DAG: VSHUFPD $0x1, X1, X2, X0
// LLVM-ASM-AMD64-DAG: VSHUFPD $0x5, Y1, Y2, Y0
// LLVM-ASM-AMD64-DAG: VSHUFPD $0x55, Z1, Z2, Z0
// LLVM-ASM-AMD64-DAG: VPERM2F128 $0x21,
// LLVM-ASM-AMD64-DAG: VPSHUFLW $0x63,
// LLVM-ASM-AMD64-DAG: VPSHUFHW $0x63,
// LLVM-ASM-AMD64-DAG: VPALIGNR $0x7, X1, X2, X0
// LLVM-ASM-AMD64-DAG: VPALIGNR $0x7, Z1, Z0, Z0

//go:noinline
func llvmImmediateConcatPermute128ScalarsFloat32x8(xConcatPermute128ScalarsFloat32x8 archsimd.Float32x8, yConcatPermute128ScalarsFloat32x8 archsimd.Float32x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x8{}
	}
	return xConcatPermute128ScalarsFloat32x8.ConcatPermute128Scalars(3, 0, yConcatPermute128ScalarsFloat32x8)
}

//go:noinline
func llvmImmediateConcatPermute128ScalarsFloat64x4(xConcatPermute128ScalarsFloat64x4 archsimd.Float64x4, yConcatPermute128ScalarsFloat64x4 archsimd.Float64x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x4{}
	}
	return xConcatPermute128ScalarsFloat64x4.ConcatPermute128Scalars(3, 0, yConcatPermute128ScalarsFloat64x4)
}

//go:noinline
func llvmImmediateConcatPermute128ScalarsInt8x32(xConcatPermute128ScalarsInt8x32 archsimd.Int8x32, yConcatPermute128ScalarsInt8x32 archsimd.Int8x32) archsimd.Int8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int8x32{}
	}
	return xConcatPermute128ScalarsInt8x32.ConcatPermute128Scalars(3, 0, yConcatPermute128ScalarsInt8x32)
}

//go:noinline
func llvmImmediateConcatPermute128ScalarsInt16x16(xConcatPermute128ScalarsInt16x16 archsimd.Int16x16, yConcatPermute128ScalarsInt16x16 archsimd.Int16x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return xConcatPermute128ScalarsInt16x16.ConcatPermute128Scalars(3, 0, yConcatPermute128ScalarsInt16x16)
}

//go:noinline
func llvmImmediateConcatPermute128ScalarsInt32x8(xConcatPermute128ScalarsInt32x8 archsimd.Int32x8, yConcatPermute128ScalarsInt32x8 archsimd.Int32x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return xConcatPermute128ScalarsInt32x8.ConcatPermute128Scalars(3, 0, yConcatPermute128ScalarsInt32x8)
}

//go:noinline
func llvmImmediateConcatPermute128ScalarsInt64x4(xConcatPermute128ScalarsInt64x4 archsimd.Int64x4, yConcatPermute128ScalarsInt64x4 archsimd.Int64x4) archsimd.Int64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x4{}
	}
	return xConcatPermute128ScalarsInt64x4.ConcatPermute128Scalars(3, 0, yConcatPermute128ScalarsInt64x4)
}

//go:noinline
func llvmImmediateConcatPermute128ScalarsUint8x32(xConcatPermute128ScalarsUint8x32 archsimd.Uint8x32, yConcatPermute128ScalarsUint8x32 archsimd.Uint8x32) archsimd.Uint8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x32{}
	}
	return xConcatPermute128ScalarsUint8x32.ConcatPermute128Scalars(3, 0, yConcatPermute128ScalarsUint8x32)
}

//go:noinline
func llvmImmediateConcatPermute128ScalarsUint16x16(xConcatPermute128ScalarsUint16x16 archsimd.Uint16x16, yConcatPermute128ScalarsUint16x16 archsimd.Uint16x16) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return xConcatPermute128ScalarsUint16x16.ConcatPermute128Scalars(3, 0, yConcatPermute128ScalarsUint16x16)
}

//go:noinline
func llvmImmediateConcatPermute128ScalarsUint32x8(xConcatPermute128ScalarsUint32x8 archsimd.Uint32x8, yConcatPermute128ScalarsUint32x8 archsimd.Uint32x8) archsimd.Uint32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x8{}
	}
	return xConcatPermute128ScalarsUint32x8.ConcatPermute128Scalars(3, 0, yConcatPermute128ScalarsUint32x8)
}

//go:noinline
func llvmImmediateConcatPermute128ScalarsUint64x4(xConcatPermute128ScalarsUint64x4 archsimd.Uint64x4, yConcatPermute128ScalarsUint64x4 archsimd.Uint64x4) archsimd.Uint64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x4{}
	}
	return xConcatPermute128ScalarsUint64x4.ConcatPermute128Scalars(3, 0, yConcatPermute128ScalarsUint64x4)
}

//go:noinline
func llvmImmediateConcatShiftBytesRightGroupedUint8x32(xConcatShiftBytesRightGroupedUint8x32 archsimd.Uint8x32, yConcatShiftBytesRightGroupedUint8x32 archsimd.Uint8x32) archsimd.Uint8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x32{}
	}
	return xConcatShiftBytesRightGroupedUint8x32.ConcatShiftBytesRightGrouped(yConcatShiftBytesRightGroupedUint8x32, 7)
}

//go:noinline
func llvmImmediateConcatShiftBytesRightGroupedUint8x64(xConcatShiftBytesRightGroupedUint8x64 archsimd.Uint8x64, yConcatShiftBytesRightGroupedUint8x64 archsimd.Uint8x64) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x64{}
	}
	return xConcatShiftBytesRightGroupedUint8x64.ConcatShiftBytesRightGrouped(yConcatShiftBytesRightGroupedUint8x64, 7)
}

//go:noinline
func llvmImmediateConcatShiftBytesRightUint8x16(xConcatShiftBytesRightUint8x16 archsimd.Uint8x16, yConcatShiftBytesRightUint8x16 archsimd.Uint8x16) archsimd.Uint8x16 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint8x16{}
	}
	return xConcatShiftBytesRightUint8x16.ConcatShiftBytesRight(yConcatShiftBytesRightUint8x16, 7)
}

//go:noinline
func llvmImmediateconcatSelectedConstantFloat32x4(xconcatSelectedConstantFloat32x4 archsimd.Float32x4, yconcatSelectedConstantFloat32x4 archsimd.Float32x4) archsimd.Float32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x4{}
	}
	return xconcatSelectedConstantFloat32x4.ConcatPermuteScalars(2, 0, 5, 7, yconcatSelectedConstantFloat32x4)
}

//go:noinline
func llvmImmediateconcatSelectedConstantFloat64x2(xconcatSelectedConstantFloat64x2 archsimd.Float64x2, yconcatSelectedConstantFloat64x2 archsimd.Float64x2) archsimd.Float64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x2{}
	}
	return xconcatSelectedConstantFloat64x2.ConcatPermuteScalars(1, 2, yconcatSelectedConstantFloat64x2)
}

//go:noinline
func llvmImmediateconcatSelectedConstantGroupedFloat32x8(xconcatSelectedConstantGroupedFloat32x8 archsimd.Float32x8, yconcatSelectedConstantGroupedFloat32x8 archsimd.Float32x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x8{}
	}
	return xconcatSelectedConstantGroupedFloat32x8.ConcatPermuteScalarsGrouped(2, 0, 5, 7, yconcatSelectedConstantGroupedFloat32x8)
}

//go:noinline
func llvmImmediateconcatSelectedConstantGroupedFloat32x16(xconcatSelectedConstantGroupedFloat32x16 archsimd.Float32x16, yconcatSelectedConstantGroupedFloat32x16 archsimd.Float32x16) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return xconcatSelectedConstantGroupedFloat32x16.ConcatPermuteScalarsGrouped(2, 0, 5, 7, yconcatSelectedConstantGroupedFloat32x16)
}

//go:noinline
func llvmImmediateconcatSelectedConstantGroupedFloat64x4(xconcatSelectedConstantGroupedFloat64x4 archsimd.Float64x4, yconcatSelectedConstantGroupedFloat64x4 archsimd.Float64x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x4{}
	}
	return xconcatSelectedConstantGroupedFloat64x4.ConcatPermuteScalarsGrouped(1, 2, yconcatSelectedConstantGroupedFloat64x4)
}

//go:noinline
func llvmImmediateconcatSelectedConstantGroupedFloat64x8(xconcatSelectedConstantGroupedFloat64x8 archsimd.Float64x8, yconcatSelectedConstantGroupedFloat64x8 archsimd.Float64x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return xconcatSelectedConstantGroupedFloat64x8.ConcatPermuteScalarsGrouped(1, 2, yconcatSelectedConstantGroupedFloat64x8)
}

//go:noinline
func llvmImmediateconcatSelectedConstantGroupedInt32x8(xconcatSelectedConstantGroupedInt32x8 archsimd.Int32x8, yconcatSelectedConstantGroupedInt32x8 archsimd.Int32x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x8{}
	}
	return xconcatSelectedConstantGroupedInt32x8.ConcatPermuteScalarsGrouped(2, 0, 5, 7, yconcatSelectedConstantGroupedInt32x8)
}

//go:noinline
func llvmImmediateconcatSelectedConstantGroupedInt32x16(xconcatSelectedConstantGroupedInt32x16 archsimd.Int32x16, yconcatSelectedConstantGroupedInt32x16 archsimd.Int32x16) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return xconcatSelectedConstantGroupedInt32x16.ConcatPermuteScalarsGrouped(2, 0, 5, 7, yconcatSelectedConstantGroupedInt32x16)
}

//go:noinline
func llvmImmediateconcatSelectedConstantGroupedInt64x4(xconcatSelectedConstantGroupedInt64x4 archsimd.Int64x4, yconcatSelectedConstantGroupedInt64x4 archsimd.Int64x4) archsimd.Int64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Int64x4{}
	}
	return xconcatSelectedConstantGroupedInt64x4.ConcatPermuteScalarsGrouped(1, 2, yconcatSelectedConstantGroupedInt64x4)
}

//go:noinline
func llvmImmediateconcatSelectedConstantGroupedInt64x8(xconcatSelectedConstantGroupedInt64x8 archsimd.Int64x8, yconcatSelectedConstantGroupedInt64x8 archsimd.Int64x8) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return xconcatSelectedConstantGroupedInt64x8.ConcatPermuteScalarsGrouped(1, 2, yconcatSelectedConstantGroupedInt64x8)
}

//go:noinline
func llvmImmediateconcatSelectedConstantGroupedUint32x8(xconcatSelectedConstantGroupedUint32x8 archsimd.Uint32x8, yconcatSelectedConstantGroupedUint32x8 archsimd.Uint32x8) archsimd.Uint32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint32x8{}
	}
	return xconcatSelectedConstantGroupedUint32x8.ConcatPermuteScalarsGrouped(2, 0, 5, 7, yconcatSelectedConstantGroupedUint32x8)
}

//go:noinline
func llvmImmediateconcatSelectedConstantGroupedUint32x16(xconcatSelectedConstantGroupedUint32x16 archsimd.Uint32x16, yconcatSelectedConstantGroupedUint32x16 archsimd.Uint32x16) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return xconcatSelectedConstantGroupedUint32x16.ConcatPermuteScalarsGrouped(2, 0, 5, 7, yconcatSelectedConstantGroupedUint32x16)
}

//go:noinline
func llvmImmediateconcatSelectedConstantGroupedUint64x4(xconcatSelectedConstantGroupedUint64x4 archsimd.Uint64x4, yconcatSelectedConstantGroupedUint64x4 archsimd.Uint64x4) archsimd.Uint64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint64x4{}
	}
	return xconcatSelectedConstantGroupedUint64x4.ConcatPermuteScalarsGrouped(1, 2, yconcatSelectedConstantGroupedUint64x4)
}

//go:noinline
func llvmImmediateconcatSelectedConstantGroupedUint64x8(xconcatSelectedConstantGroupedUint64x8 archsimd.Uint64x8, yconcatSelectedConstantGroupedUint64x8 archsimd.Uint64x8) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return xconcatSelectedConstantGroupedUint64x8.ConcatPermuteScalarsGrouped(1, 2, yconcatSelectedConstantGroupedUint64x8)
}

//go:noinline
func llvmImmediateconcatSelectedConstantInt32x4(xconcatSelectedConstantInt32x4 archsimd.Int32x4, yconcatSelectedConstantInt32x4 archsimd.Int32x4) archsimd.Int32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x4{}
	}
	return xconcatSelectedConstantInt32x4.ConcatPermuteScalars(2, 0, 5, 7, yconcatSelectedConstantInt32x4)
}

//go:noinline
func llvmImmediateconcatSelectedConstantInt64x2(xconcatSelectedConstantInt64x2 archsimd.Int64x2, yconcatSelectedConstantInt64x2 archsimd.Int64x2) archsimd.Int64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Int64x2{}
	}
	return xconcatSelectedConstantInt64x2.ConcatPermuteScalars(1, 2, yconcatSelectedConstantInt64x2)
}

//go:noinline
func llvmImmediateconcatSelectedConstantUint32x4(xconcatSelectedConstantUint32x4 archsimd.Uint32x4, yconcatSelectedConstantUint32x4 archsimd.Uint32x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint32x4{}
	}
	return xconcatSelectedConstantUint32x4.ConcatPermuteScalars(2, 0, 5, 7, yconcatSelectedConstantUint32x4)
}

//go:noinline
func llvmImmediateconcatSelectedConstantUint64x2(xconcatSelectedConstantUint64x2 archsimd.Uint64x2, yconcatSelectedConstantUint64x2 archsimd.Uint64x2) archsimd.Uint64x2 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint64x2{}
	}
	return xconcatSelectedConstantUint64x2.ConcatPermuteScalars(1, 2, yconcatSelectedConstantUint64x2)
}

//go:noinline
func llvmImmediatepermuteScalarsGroupedInt32x8(xpermuteScalarsGroupedInt32x8 archsimd.Int32x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return xpermuteScalarsGroupedInt32x8.PermuteScalarsGrouped(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsGroupedInt32x16(xpermuteScalarsGroupedInt32x16 archsimd.Int32x16) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return xpermuteScalarsGroupedInt32x16.PermuteScalarsGrouped(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsGroupedUint32x8(xpermuteScalarsGroupedUint32x8 archsimd.Uint32x8) archsimd.Uint32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x8{}
	}
	return xpermuteScalarsGroupedUint32x8.PermuteScalarsGrouped(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsGroupedUint32x16(xpermuteScalarsGroupedUint32x16 archsimd.Uint32x16) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return xpermuteScalarsGroupedUint32x16.PermuteScalarsGrouped(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsHiGroupedInt16x16(xpermuteScalarsHiGroupedInt16x16 archsimd.Int16x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return xpermuteScalarsHiGroupedInt16x16.PermuteScalarsHiGrouped(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsHiGroupedInt16x32(xpermuteScalarsHiGroupedInt16x32 archsimd.Int16x32) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return xpermuteScalarsHiGroupedInt16x32.PermuteScalarsHiGrouped(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsHiGroupedUint16x16(xpermuteScalarsHiGroupedUint16x16 archsimd.Uint16x16) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return xpermuteScalarsHiGroupedUint16x16.PermuteScalarsHiGrouped(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsHiGroupedUint16x32(xpermuteScalarsHiGroupedUint16x32 archsimd.Uint16x32) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return xpermuteScalarsHiGroupedUint16x32.PermuteScalarsHiGrouped(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsHiInt16x8(xpermuteScalarsHiInt16x8 archsimd.Int16x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Int16x8{}
	}
	return xpermuteScalarsHiInt16x8.PermuteScalarsHi(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsHiUint16x8(xpermuteScalarsHiUint16x8 archsimd.Uint16x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint16x8{}
	}
	return xpermuteScalarsHiUint16x8.PermuteScalarsHi(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsInt32x4(xpermuteScalarsInt32x4 archsimd.Int32x4) archsimd.Int32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x4{}
	}
	return xpermuteScalarsInt32x4.PermuteScalars(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsLoGroupedInt16x16(xpermuteScalarsLoGroupedInt16x16 archsimd.Int16x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return xpermuteScalarsLoGroupedInt16x16.PermuteScalarsLoGrouped(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsLoGroupedInt16x32(xpermuteScalarsLoGroupedInt16x32 archsimd.Int16x32) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return xpermuteScalarsLoGroupedInt16x32.PermuteScalarsLoGrouped(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsLoGroupedUint16x16(xpermuteScalarsLoGroupedUint16x16 archsimd.Uint16x16) archsimd.Uint16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint16x16{}
	}
	return xpermuteScalarsLoGroupedUint16x16.PermuteScalarsLoGrouped(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsLoGroupedUint16x32(xpermuteScalarsLoGroupedUint16x32 archsimd.Uint16x32) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return xpermuteScalarsLoGroupedUint16x32.PermuteScalarsLoGrouped(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsLoInt16x8(xpermuteScalarsLoInt16x8 archsimd.Int16x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Int16x8{}
	}
	return xpermuteScalarsLoInt16x8.PermuteScalarsLo(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsLoUint16x8(xpermuteScalarsLoUint16x8 archsimd.Uint16x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint16x8{}
	}
	return xpermuteScalarsLoUint16x8.PermuteScalarsLo(3, 0, 2, 1)
}

//go:noinline
func llvmImmediatepermuteScalarsUint32x4(xpermuteScalarsUint32x4 archsimd.Uint32x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint32x4{}
	}
	return xpermuteScalarsUint32x4.PermuteScalars(3, 0, 2, 1)
}
