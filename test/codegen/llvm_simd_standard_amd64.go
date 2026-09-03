// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && amd64

package codegen

import "simd/archsimd"

// LLVM-AMD64: load i8, ptr getelementptr {{.*}}!goallc.cpu.guard ![[VPOPCNTDQ:[0-9]+]]
// LLVM-AMD64: call <4 x i32> @llvm.ctpop.v4i32{{.*}}!goallc.cpu.requires ![[VPOPCNTDQ]]
// LLVM-AMD64-DAG: ![[VPOPCNTDQ]] = !{!"x86.avx512vpopcntdq"}
//
//go:noinline
func llvmSIMDStandardOnesCount32(x archsimd.Uint32x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVX512VPOPCNTDQ() {
		return x
	}
	return x.OnesCount()
}
