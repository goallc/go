// asmcheck

//go:build goexperiment.simd && arm64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-ARM64-DAG: define {{.*}} @codegen.fmaFloat32x4(
// LLVM-ARM64-DAG: call <4 x float> @llvm.fma.v4f32
// LLVM-OPT-ARM64-DAG: call <4 x float> @llvm.fma.v4f32
//
//go:noinline
func fmaFloat32x4(x, y, z archsimd.Float32x4) archsimd.Float32x4 {
	return x.MulAdd(y, z)
}

// LLVM-ARM64-DAG: define {{.*}} @codegen.fmaFloat64x2(
// LLVM-ARM64-DAG: call <2 x double> @llvm.fma.v2f64
// LLVM-OPT-ARM64-DAG: call <2 x double> @llvm.fma.v2f64
//
//go:noinline
func fmaFloat64x2(x, y, z archsimd.Float64x2) archsimd.Float64x2 {
	return x.MulAdd(y, z)
}

// LLVM-ASM-ARM64-DAG: VFMLA
