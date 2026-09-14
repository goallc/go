// asmcheck

//go:build goexperiment.simd && arm64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-ARM64-DAG: define {{.*}} @codegen.mulAddInt8x16(
// LLVM-ARM64-DAG: mul <16 x i8>
// LLVM-ARM64-DAG: add <16 x i8>
// LLVM-OPT-ARM64-DAG: define {{.*}} @codegen.mulAddInt8x16(
// LLVM-OPT-ARM64-DAG: mul <16 x i8>
// LLVM-OPT-ARM64-DAG: add <16 x i8>
// LLVM-ASM-ARM64-DAG: VMLA V
//
//go:noinline
func mulAddInt8x16(x, y, z archsimd.Int8x16) archsimd.Int8x16 {
	return x.MulAdd(y, z)
}

// LLVM-ARM64-DAG: define {{.*}} @codegen.mulAddUint8x16(
// LLVM-ARM64-DAG: mul <16 x i8>
// LLVM-ARM64-DAG: add <16 x i8>
// LLVM-OPT-ARM64-DAG: define {{.*}} @codegen.mulAddUint8x16(
// LLVM-OPT-ARM64-DAG: mul <16 x i8>
// LLVM-OPT-ARM64-DAG: add <16 x i8>
// LLVM-ASM-ARM64-DAG: VMLA V
//
//go:noinline
func mulAddUint8x16(x, y, z archsimd.Uint8x16) archsimd.Uint8x16 {
	return x.MulAdd(y, z)
}

// LLVM-ARM64-DAG: define {{.*}} @codegen.mulAddInt16x8(
// LLVM-ARM64-DAG: mul <8 x i16>
// LLVM-ARM64-DAG: add <8 x i16>
// LLVM-OPT-ARM64-DAG: define {{.*}} @codegen.mulAddInt16x8(
// LLVM-OPT-ARM64-DAG: mul <8 x i16>
// LLVM-OPT-ARM64-DAG: add <8 x i16>
// LLVM-ASM-ARM64-DAG: VMLA V
//
//go:noinline
func mulAddInt16x8(x, y, z archsimd.Int16x8) archsimd.Int16x8 {
	return x.MulAdd(y, z)
}

// LLVM-ARM64-DAG: define {{.*}} @codegen.mulAddUint16x8(
// LLVM-ARM64-DAG: mul <8 x i16>
// LLVM-ARM64-DAG: add <8 x i16>
// LLVM-OPT-ARM64-DAG: define {{.*}} @codegen.mulAddUint16x8(
// LLVM-OPT-ARM64-DAG: mul <8 x i16>
// LLVM-OPT-ARM64-DAG: add <8 x i16>
// LLVM-ASM-ARM64-DAG: VMLA V
//
//go:noinline
func mulAddUint16x8(x, y, z archsimd.Uint16x8) archsimd.Uint16x8 {
	return x.MulAdd(y, z)
}

// LLVM-ARM64-DAG: define {{.*}} @codegen.mulAddInt32x4(
// LLVM-ARM64-DAG: mul <4 x i32>
// LLVM-ARM64-DAG: add <4 x i32>
// LLVM-OPT-ARM64-DAG: define {{.*}} @codegen.mulAddInt32x4(
// LLVM-OPT-ARM64-DAG: mul <4 x i32>
// LLVM-OPT-ARM64-DAG: add <4 x i32>
// LLVM-ASM-ARM64-DAG: VMLA V
//
//go:noinline
func mulAddInt32x4(x, y, z archsimd.Int32x4) archsimd.Int32x4 {
	return x.MulAdd(y, z)
}

// LLVM-ARM64-DAG: define {{.*}} @codegen.mulAddUint32x4(
// LLVM-ARM64-DAG: mul <4 x i32>
// LLVM-ARM64-DAG: add <4 x i32>
// LLVM-OPT-ARM64-DAG: define {{.*}} @codegen.mulAddUint32x4(
// LLVM-OPT-ARM64-DAG: mul <4 x i32>
// LLVM-OPT-ARM64-DAG: add <4 x i32>
// LLVM-ASM-ARM64-DAG: VMLA V
//
//go:noinline
func mulAddUint32x4(x, y, z archsimd.Uint32x4) archsimd.Uint32x4 {
	return x.MulAdd(y, z)
}
