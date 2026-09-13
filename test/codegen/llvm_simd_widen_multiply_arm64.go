// asmcheck

//go:build goexperiment.simd && arm64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-ARM64-DAG: define {{.*}} @codegen.widenInt8x16(
// LLVM-ARM64-DAG: shufflevector <16 x i8>
// LLVM-ARM64-DAG: sext <8 x i8> {{.*}} to <8 x i16>
// LLVM-ARM64-DAG: mul <8 x i16>
//
//go:noinline
func widenInt8x16(x, y archsimd.Int8x16) archsimd.Int16x8 {
	return x.MulWidenLo(y)
}

// LLVM-ARM64-DAG: define {{.*}} @codegen.widenInt16x8(
// LLVM-ARM64-DAG: shufflevector <8 x i16>
// LLVM-ARM64-DAG: sext <4 x i16> {{.*}} to <4 x i32>
// LLVM-ARM64-DAG: mul <4 x i32>
//
//go:noinline
func widenInt16x8(x, y archsimd.Int16x8) archsimd.Int32x4 {
	return x.MulWidenLo(y)
}

// LLVM-ARM64-DAG: define {{.*}} @codegen.widenInt32x4(
// LLVM-ARM64-DAG: shufflevector <4 x i32>
// LLVM-ARM64-DAG: sext <2 x i32> {{.*}} to <2 x i64>
// LLVM-ARM64-DAG: mul <2 x i64>
//
//go:noinline
func widenInt32x4(x, y archsimd.Int32x4) archsimd.Int64x2 {
	return x.MulWidenLo(y)
}

// LLVM-ARM64-DAG: define {{.*}} @codegen.widenUint8x16(
// LLVM-ARM64-DAG: shufflevector <16 x i8>
// LLVM-ARM64-DAG: zext <8 x i8> {{.*}} to <8 x i16>
// LLVM-ARM64-DAG: mul <8 x i16>
//
//go:noinline
func widenUint8x16(x, y archsimd.Uint8x16) archsimd.Uint16x8 {
	return x.MulWidenLo(y)
}

// LLVM-ARM64-DAG: define {{.*}} @codegen.widenUint16x8(
// LLVM-ARM64-DAG: shufflevector <8 x i16>
// LLVM-ARM64-DAG: zext <4 x i16> {{.*}} to <4 x i32>
// LLVM-ARM64-DAG: mul <4 x i32>
//
//go:noinline
func widenUint16x8(x, y archsimd.Uint16x8) archsimd.Uint32x4 {
	return x.MulWidenLo(y)
}

// LLVM-ARM64-DAG: define {{.*}} @codegen.widenUint32x4(
// LLVM-ARM64-DAG: shufflevector <4 x i32>
// LLVM-ARM64-DAG: zext <2 x i32> {{.*}} to <2 x i64>
// LLVM-ARM64-DAG: mul <2 x i64>
//
//go:noinline
func widenUint32x4(x, y archsimd.Uint32x4) archsimd.Uint64x2 {
	return x.MulWidenLo(y)
}

// LLVM-ASM-ARM64-DAG: SMULL
// LLVM-ASM-ARM64-DAG: UMULL
