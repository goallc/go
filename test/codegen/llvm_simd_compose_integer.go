// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && (amd64 || arm64)

package codegen

import (
	"runtime"
	"simd/archsimd"
)

// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmSIMDComposeAverageUnsigned(
// LLVM-AMD64-DAG: xor <16 x i8>
// LLVM-AMD64-DAG: lshr <16 x i8>
// LLVM-AMD64-DAG: or <16 x i8>
// LLVM-AMD64-DAG: sub <16 x i8>
// LLVM-AMD64-DAG: "goallc.cpu.feature-floor"="x86.avx"
// LLVM-ASM-AMD64-LABEL: TEXT codegen.llvmSIMDComposeAverageUnsigned(SB)
// LLVM-ASM-AMD64: VPAVGB
// LLVM-ARM64-DAG: xor <16 x i8>
// LLVM-ARM64-DAG: lshr <16 x i8>
// LLVM-ARM64-DAG: or <16 x i8>
// LLVM-ARM64-DAG: sub <16 x i8>
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmSIMDComposeAverageUnsigned(SB)
// LLVM-ASM-ARM64: VURHADD
//
//go:noinline
func llvmSIMDComposeAverageUnsigned(x, y archsimd.Uint8x16) archsimd.Uint8x16 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return x
	}
	return x.Average(y)
}
