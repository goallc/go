// asmcheck

//go:build goexperiment.simd && arm64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-ARM64-DAG: call <16 x i8> @llvm.aarch64.neon.pmull64(i64
// LLVM-ARM64-DAG: !{!"arm64.pmull"}
// LLVM-OPT-ARM64-DAG: define internal {{.*}} @"codegen.pmullEven<goallc.fmv.pmull>"
// LLVM-OPT-ARM64-DAG: define internal {{.*}} @"codegen.pmullOdd<goallc.fmv.pmull>"
// LLVM-OPT-ARM64-DAG: "target-features"="{{[^"]*}}+aes{{[^"]*}}"
// LLVM-ASM-ARM64-DAG: VPMULL V
// LLVM-ASM-ARM64-DAG: VPMULL2 V
//
//go:noinline
func pmullEven(x, y archsimd.Uint64x2) archsimd.Uint64x2 {
	if archsimd.ARM64.PMULL() {
		return x.CarrylessMultiplyEven(y)
	}
	return x
}

//go:noinline
func pmullOdd(x, y archsimd.Uint64x2) archsimd.Uint64x2 {
	if !archsimd.ARM64.PMULL() {
		return y
	}
	return x.CarrylessMultiplyOdd(y)
}
