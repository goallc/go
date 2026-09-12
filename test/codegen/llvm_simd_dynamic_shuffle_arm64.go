// asmcheck

//go:build goexperiment.simd && arm64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-ASM-ARM64-DAG: VTBL V2.B16, [V0.B16], V0.B16

// LLVM-ARM64-DAG: call <16 x i8> @llvm.aarch64.neon.tbl1.v16i8(

// LLVM-ARM64-DAG: define {{.*}} @codegen.llvmDynamicLookupOrZeroInt8x16(
// LLVM-ARM64-DAG: define {{.*}} @codegen.llvmDynamicLookupOrZeroUint8x16(

//go:noinline
func llvmDynamicLookupOrZeroInt8x16(x, y archsimd.Int8x16, indices archsimd.Int8x16) archsimd.Int8x16 {
	return x.LookupOrZero(indices)
}

//go:noinline
func llvmDynamicLookupOrZeroUint8x16(x, y archsimd.Uint8x16, indices archsimd.Uint8x16) archsimd.Uint8x16 {
	return x.LookupOrZero(indices)
}
