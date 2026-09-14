// asmcheck

//go:build goexperiment.simd && arm64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-ARM64-DAG: call <16 x i8> @llvm.aarch64.neon.sshl.v16i8(
// LLVM-OPT-ARM64-DAG: call <16 x i8> @llvm.aarch64.neon.sshl.v16i8(
// LLVM-ASM-ARM64-DAG: VSSHL V
//
//go:noinline
func ShiftInt8x16(x archsimd.Int8x16, count archsimd.Int8x16) archsimd.Int8x16 {
	return x.Shift(count)
}

// LLVM-ARM64-DAG: call <16 x i8> @llvm.aarch64.neon.sqshl.v16i8(
// LLVM-OPT-ARM64-DAG: call <16 x i8> @llvm.aarch64.neon.sqshl.v16i8(
// LLVM-ASM-ARM64-DAG: VSQSHL V
//
//go:noinline
func ShiftSaturatedInt8x16(x archsimd.Int8x16, count archsimd.Int8x16) archsimd.Int8x16 {
	return x.ShiftSaturated(count)
}

// LLVM-ARM64-DAG: call <16 x i8> @llvm.aarch64.neon.ushl.v16i8(
// LLVM-OPT-ARM64-DAG: call <16 x i8> @llvm.aarch64.neon.ushl.v16i8(
// LLVM-ASM-ARM64-DAG: VUSHL V
//
//go:noinline
func ShiftUint8x16(x archsimd.Uint8x16, count archsimd.Int8x16) archsimd.Uint8x16 {
	return x.Shift(count)
}

// LLVM-ARM64-DAG: call <16 x i8> @llvm.aarch64.neon.uqshl.v16i8(
// LLVM-OPT-ARM64-DAG: call <16 x i8> @llvm.aarch64.neon.uqshl.v16i8(
// LLVM-ASM-ARM64-DAG: VUQSHL V
//
//go:noinline
func ShiftSaturatedUint8x16(x archsimd.Uint8x16, count archsimd.Int8x16) archsimd.Uint8x16 {
	return x.ShiftSaturated(count)
}

// LLVM-ARM64-DAG: call <8 x i16> @llvm.aarch64.neon.sshl.v8i16(
// LLVM-OPT-ARM64-DAG: call <8 x i16> @llvm.aarch64.neon.sshl.v8i16(
// LLVM-ASM-ARM64-DAG: VSSHL V
//
//go:noinline
func ShiftInt16x8(x archsimd.Int16x8, count archsimd.Int16x8) archsimd.Int16x8 {
	return x.Shift(count)
}

// LLVM-ARM64-DAG: call <8 x i16> @llvm.aarch64.neon.sqshl.v8i16(
// LLVM-OPT-ARM64-DAG: call <8 x i16> @llvm.aarch64.neon.sqshl.v8i16(
// LLVM-ASM-ARM64-DAG: VSQSHL V
//
//go:noinline
func ShiftSaturatedInt16x8(x archsimd.Int16x8, count archsimd.Int16x8) archsimd.Int16x8 {
	return x.ShiftSaturated(count)
}

// LLVM-ARM64-DAG: call <8 x i16> @llvm.aarch64.neon.ushl.v8i16(
// LLVM-OPT-ARM64-DAG: call <8 x i16> @llvm.aarch64.neon.ushl.v8i16(
// LLVM-ASM-ARM64-DAG: VUSHL V
//
//go:noinline
func ShiftUint16x8(x archsimd.Uint16x8, count archsimd.Int16x8) archsimd.Uint16x8 {
	return x.Shift(count)
}

// LLVM-ARM64-DAG: call <8 x i16> @llvm.aarch64.neon.uqshl.v8i16(
// LLVM-OPT-ARM64-DAG: call <8 x i16> @llvm.aarch64.neon.uqshl.v8i16(
// LLVM-ASM-ARM64-DAG: VUQSHL V
//
//go:noinline
func ShiftSaturatedUint16x8(x archsimd.Uint16x8, count archsimd.Int16x8) archsimd.Uint16x8 {
	return x.ShiftSaturated(count)
}

// LLVM-ARM64-DAG: call <4 x i32> @llvm.aarch64.neon.sshl.v4i32(
// LLVM-OPT-ARM64-DAG: call <4 x i32> @llvm.aarch64.neon.sshl.v4i32(
// LLVM-ASM-ARM64-DAG: VSSHL V
//
//go:noinline
func ShiftInt32x4(x archsimd.Int32x4, count archsimd.Int32x4) archsimd.Int32x4 {
	return x.Shift(count)
}

// LLVM-ARM64-DAG: call <4 x i32> @llvm.aarch64.neon.sqshl.v4i32(
// LLVM-OPT-ARM64-DAG: call <4 x i32> @llvm.aarch64.neon.sqshl.v4i32(
// LLVM-ASM-ARM64-DAG: VSQSHL V
//
//go:noinline
func ShiftSaturatedInt32x4(x archsimd.Int32x4, count archsimd.Int32x4) archsimd.Int32x4 {
	return x.ShiftSaturated(count)
}

// LLVM-ARM64-DAG: call <4 x i32> @llvm.aarch64.neon.ushl.v4i32(
// LLVM-OPT-ARM64-DAG: call <4 x i32> @llvm.aarch64.neon.ushl.v4i32(
// LLVM-ASM-ARM64-DAG: VUSHL V
//
//go:noinline
func ShiftUint32x4(x archsimd.Uint32x4, count archsimd.Int32x4) archsimd.Uint32x4 {
	return x.Shift(count)
}

// LLVM-ARM64-DAG: call <4 x i32> @llvm.aarch64.neon.uqshl.v4i32(
// LLVM-OPT-ARM64-DAG: call <4 x i32> @llvm.aarch64.neon.uqshl.v4i32(
// LLVM-ASM-ARM64-DAG: VUQSHL V
//
//go:noinline
func ShiftSaturatedUint32x4(x archsimd.Uint32x4, count archsimd.Int32x4) archsimd.Uint32x4 {
	return x.ShiftSaturated(count)
}

// LLVM-ARM64-DAG: call <2 x i64> @llvm.aarch64.neon.sshl.v2i64(
// LLVM-OPT-ARM64-DAG: call <2 x i64> @llvm.aarch64.neon.sshl.v2i64(
// LLVM-ASM-ARM64-DAG: VSSHL V
//
//go:noinline
func ShiftInt64x2(x archsimd.Int64x2, count archsimd.Int64x2) archsimd.Int64x2 {
	return x.Shift(count)
}

// LLVM-ARM64-DAG: call <2 x i64> @llvm.aarch64.neon.sqshl.v2i64(
// LLVM-OPT-ARM64-DAG: call <2 x i64> @llvm.aarch64.neon.sqshl.v2i64(
// LLVM-ASM-ARM64-DAG: VSQSHL V
//
//go:noinline
func ShiftSaturatedInt64x2(x archsimd.Int64x2, count archsimd.Int64x2) archsimd.Int64x2 {
	return x.ShiftSaturated(count)
}

// LLVM-ARM64-DAG: call <2 x i64> @llvm.aarch64.neon.ushl.v2i64(
// LLVM-OPT-ARM64-DAG: call <2 x i64> @llvm.aarch64.neon.ushl.v2i64(
// LLVM-ASM-ARM64-DAG: VUSHL V
//
//go:noinline
func ShiftUint64x2(x archsimd.Uint64x2, count archsimd.Int64x2) archsimd.Uint64x2 {
	return x.Shift(count)
}

// LLVM-ARM64-DAG: call <2 x i64> @llvm.aarch64.neon.uqshl.v2i64(
// LLVM-OPT-ARM64-DAG: call <2 x i64> @llvm.aarch64.neon.uqshl.v2i64(
// LLVM-ASM-ARM64-DAG: VUQSHL V
//
//go:noinline
func ShiftSaturatedUint64x2(x archsimd.Uint64x2, count archsimd.Int64x2) archsimd.Uint64x2 {
	return x.ShiftSaturated(count)
}
