// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && arm64

package codegen

import "simd/archsimd"

// LLVM-ARM64-DAG: xor <16 x i8>
// LLVM-ARM64-DAG: ashr <16 x i8>
// LLVM-ARM64-DAG: or <16 x i8>
// LLVM-ARM64-DAG: sub <16 x i8>
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmSIMDComposeAverageSigned(SB)
// LLVM-ASM-ARM64: VSRHADD
//
//go:noinline
func llvmSIMDComposeAverageSigned(x, y archsimd.Int8x16) archsimd.Int8x16 {
	return x.Average(y)
}

// LLVM-ARM64-DAG: ashr <8 x i16>
// LLVM-ARM64-DAG: xor <8 x i16>
// LLVM-ARM64-DAG: call <8 x i16> @llvm.ctlz.v8i16({{.*}}, i1 false)
// LLVM-ARM64-DAG: sub <8 x i16>
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmSIMDComposeLeadingSignBits(SB)
// LLVM-ASM-ARM64: VCLS
//
//go:noinline
func llvmSIMDComposeLeadingSignBits(x archsimd.Int16x8) archsimd.Int16x8 {
	return x.LeadingSignBits()
}
