// asmcheck

//go:build goexperiment.simd && arm64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-ARM64-DAG: define {{.*}} @codegen.horizontalInt64x2ConcatAddPairs(
// LLVM-ARM64-DAG: add <2 x i64>
// LLVM-ASM-ARM64-DAG: VADDP
//
//go:noinline
func horizontalInt64x2ConcatAddPairs(x, y archsimd.Int64x2) archsimd.Int64x2 {
	return x.ConcatAddPairs(y)
}

// LLVM-ARM64-DAG: define {{.*}} @codegen.horizontalFloat32x4ConcatAddPairs(
// LLVM-ARM64-DAG: fadd <4 x float>
// LLVM-ASM-ARM64-DAG: VFADDP
//
//go:noinline
func horizontalFloat32x4ConcatAddPairs(x, y archsimd.Float32x4) archsimd.Float32x4 {
	return x.ConcatAddPairs(y)
}
