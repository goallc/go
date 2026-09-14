// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-AMD64-DAG: define {{.*}} @codegen.funnelRotateLeft128(
// LLVM-AMD64-DAG: call {{.*}} @llvm.fshl.v4i32
// LLVM-ASM-AMD64-DAG: VPROLVD
//
//go:noinline
func funnelRotateLeft128(x, y archsimd.Int32x4) archsimd.Int32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x4{}
	}
	return x.RotateLeft(y)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.funnelRotateRight256(
// LLVM-AMD64-DAG: call {{.*}} @llvm.fshr.v4i64
// LLVM-ASM-AMD64-DAG: VPRORVQ
//
//go:noinline
func funnelRotateRight256(x, y archsimd.Uint64x4) archsimd.Uint64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x4{}
	}
	return x.RotateRight(y)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.funnelVariableLeft512(
// LLVM-AMD64-DAG: call {{.*}} @llvm.fshl.v32i16
// LLVM-ASM-AMD64-DAG: VPSHLDVW
//
//go:noinline
func funnelVariableLeft512(x, y archsimd.Int16x32, c archsimd.Uint16x32) archsimd.Int16x32 {
	if !archsimd.X86.AVX512VBMI2() {
		return archsimd.Int16x32{}
	}
	return x.ShiftLeftConcatMod16(y, c)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.funnelVariableRight256(
// LLVM-AMD64-DAG: call {{.*}} @llvm.fshr.v8i32
// LLVM-ASM-AMD64-DAG: VPSHRDVD
//
//go:noinline
func funnelVariableRight256(x, y archsimd.Int32x8, c archsimd.Uint32x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX512VBMI2() {
		return archsimd.Int32x8{}
	}
	return x.ShiftRightConcatMod32(y, c)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.funnelConstantLeft128(
// LLVM-AMD64-DAG: call {{.*}} @llvm.fshl.v8i16
// LLVM-ASM-AMD64-DAG: VPSHLDW
//
//go:noinline
func funnelConstantLeft128(x, y archsimd.Uint16x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX512VBMI2() {
		return archsimd.Uint16x8{}
	}
	return x.ShiftAllLeftConcatMod16(y, 7)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.funnelConstantRight512(
// LLVM-AMD64-DAG: call {{.*}} @llvm.fshr.v8i64
// LLVM canonicalizes a right shift by 63 to a left shift by 1.
// LLVM-ASM-AMD64-DAG: VPSHLDQ $0x1
//
//go:noinline
func funnelConstantRight512(x, y archsimd.Uint64x8) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512VBMI2() {
		return archsimd.Uint64x8{}
	}
	return x.ShiftAllRightConcatMod64(y, 63)
}
