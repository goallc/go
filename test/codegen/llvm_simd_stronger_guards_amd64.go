// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && amd64

package codegen

import "simd/archsimd"

// The actual dominating guard supplies the specialization profile. The lower
// instruction requirement must not add a redundant independent profile.
// LLVM-AMD64-LABEL: define goabiinternal <32 x i8> @codegen.llvmStrongerGuard(
// LLVM-AMD64-SAME: #[[STRONG:[0-9]+]]
// LLVM-AMD64: load i8, ptr {{.*}}!goallc.cpu.guard ![[HIGH:[0-9]+]]
// LLVM-AMD64: add <32 x i8> {{.*}}!goallc.cpu.requires ![[LOW:[0-9]+]]
// LLVM-AMD64: attributes #[[STRONG]] = { {{.*}}"goallc.cpu.feature-floor"="x86.avx"{{.*}}"goallc.cpu.multiversion"="x86.avx512"
// LLVM-AMD64-DAG: ![[HIGH]] = !{!"x86.avx512"}
// LLVM-AMD64-DAG: ![[LOW]] = !{!"x86.avx2"}
// LLVM-NM-AMD64: codegen.llvmStrongerGuard.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-3: codegen.llvmStrongerGuard<1>
// LLVM-NM-AMD64-NOT: codegen.llvmStrongerGuard<1>

//go:noinline
func llvmStrongerGuard(x, y archsimd.Int8x32) archsimd.Int8x32 {
	if !archsimd.X86.AVX512() {
		return x
	}
	return x.Add(y)
}
