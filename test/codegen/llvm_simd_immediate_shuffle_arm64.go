// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && arm64

package codegen

import "simd/archsimd"

// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmImmediateConcatShiftBytesRightUint8x16(SB)
// LLVM-ASM-ARM64-NEXT: {{.*}} VEXT $7, V0.B16, V1.B16, V0.B16
// LLVM-ASM-ARM64-NEXT: {{.*}} RET

// Equivalent signed/unsigned API types have the same LLVM lane type. Check
// every function and each distinct IR routing shape; exhaustive SSA and runtime
// tests separately cover every opcode and all control values.
// LLVM-ARM64-DAG: define {{.*}} @codegen.llvmImmediateConcatShiftBytesRightUint8x16(
// LLVM-ARM64-DAG: shufflevector <16 x i8> %{{[^, ]+}}, <16 x i8> %{{[^, ]+}}, <16 x i32> <i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31, i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6>

//go:noinline
func llvmImmediateConcatShiftBytesRightUint8x16(xConcatShiftBytesRightUint8x16 archsimd.Uint8x16, yConcatShiftBytesRightUint8x16 archsimd.Uint8x16) archsimd.Uint8x16 {
	return xConcatShiftBytesRightUint8x16.ConcatShiftBytesRight(yConcatShiftBytesRightUint8x16, 7)
}
