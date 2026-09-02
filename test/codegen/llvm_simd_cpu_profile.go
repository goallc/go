// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && amd64

package codegen

import (
	"simd"
	"simd/archsimd"
)

// LLVM-AMD64-DAG: define goabiinternal <64 x i8> @"codegen.llvmSIMDMidwayAdd@simd512"(<64 x i8> %x, <64 x i8> %y) #[[SIMD512:[0-9]+]]
// LLVM-AMD64-DAG: define goabiinternal <32 x i8> @"codegen.llvmSIMDMidwayAdd@simd256"(<32 x i8> %x, <32 x i8> %y) #[[SIMD256:[0-9]+]]
// LLVM-AMD64-DAG: define goabiinternal <16 x i8> @"codegen.llvmSIMDMidwayAdd@simd128"(<16 x i8> %x, <16 x i8> %y) #[[SIMD128:[0-9]+]]
// LLVM-AMD64-DAG: define goabiinternal void @"codegen.llvmSIMDMidwayGuardedAVX2@simd0"{{.*}} #[[GUARDED_SIMD0:[0-9]+]]
// LLVM-AMD64-DAG: define goabiinternal void @"codegen.llvmSIMDMidwayGuardedAVX2@simd128"{{.*}} #[[GUARDED_SIMD128:[0-9]+]]
// LLVM-AMD64-LABEL: define goabiinternal void @codegen.llvmSIMDGeneratedAVX2(
// LLVM-AMD64: load i8, ptr getelementptr {{.*}}!goallc.cpu.guard ![[AVX2:[0-9]+]]
// LLVM-AMD64: add <32 x i8> {{.*}}!goallc.cpu.requires ![[AVX2]]
// LLVM-AMD64-DAG: ![[AVX2]] = !{!"x86.avx2"}
// LLVM-AMD64-DAG: attributes #[[SIMD512]] = { {{.*}}"goallc.cpu.feature-floor"="x86.avx512"
// LLVM-AMD64-DAG: attributes #[[SIMD256]] = { {{.*}}"goallc.cpu.feature-floor"="x86.avx2"
// LLVM-AMD64-DAG: attributes #[[SIMD128]] = { {{.*}}"goallc.cpu.feature-floor"="x86.avx"
// LLVM-AMD64-DAG: attributes #[[GUARDED_SIMD0]] = { {{.*}}"goallc.cpu.multiversion"="x86.avx2"
// LLVM-AMD64-DAG: attributes #[[GUARDED_SIMD128]] = { {{.*}}"goallc.cpu.feature-floor"="x86.avx"{{.*}}"goallc.cpu.multiversion"="x86.avx2"
// LLVM-OPT-AMD64-DAG: define goabiinternal <64 x i8> @"codegen.llvmSIMDMidwayAdd@simd512"(<64 x i8> %x, <64 x i8> %y) #[[OPT_SIMD512:[0-9]+]]
// LLVM-OPT-AMD64-DAG: define goabiinternal <32 x i8> @"codegen.llvmSIMDMidwayAdd@simd256"(<32 x i8> %x, <32 x i8> %y) #[[OPT_SIMD256:[0-9]+]]
// LLVM-OPT-AMD64-DAG: define goabiinternal <16 x i8> @"codegen.llvmSIMDMidwayAdd@simd128"(<16 x i8> %x, <16 x i8> %y) #[[OPT_SIMD128:[0-9]+]]
// LLVM-OPT-AMD64-DAG: attributes #[[OPT_SIMD512]] = { {{.*}}"target-features"="+avx,+avx2,+avx512f,+avx512cd,+avx512bw,+avx512dq,+avx512vl"
// LLVM-OPT-AMD64-DAG: attributes #[[OPT_SIMD256]] = { {{.*}}"target-features"="+avx,+avx2"
// LLVM-OPT-AMD64-DAG: attributes #[[OPT_SIMD128]] = { {{.*}}"target-features"="+avx"
// LLVM-NM-AMD64: codegen.llvmSIMDGeneratedAVX2.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-3: codegen.llvmSIMDGeneratedAVX2<1>
// LLVM-NM-AMD64-NOT: codegen.llvmSIMDGeneratedAVX2<1>
// LLVM-NM-AMD64-COUNT-5: codegen.llvmSIMDMidwayAdd
// LLVM-NM-AMD64-NOT: codegen.llvmSIMDMidwayAdd
// LLVM-NM-AMD64: codegen.llvmSIMDMidwayGuardedAVX2@simd0.goallc.fmv.slot
// LLVM-NM-AMD64: codegen.llvmSIMDMidwayGuardedAVX2@simd128.goallc.fmv.slot
// LLVM-NM-AMD64-NOT: codegen.llvmSIMDMidwayGuardedAVX2@simd256.goallc.fmv.slot
// LLVM-NM-AMD64-NOT: codegen.llvmSIMDMidwayGuardedAVX2@simd512.goallc.fmv.slot
//
//go:noinline
func llvmSIMDGeneratedAVX2(dst, x, y *[32]int8) {
	if archsimd.X86.AVX2() {
		archsimd.LoadInt8x32Array(x).Add(archsimd.LoadInt8x32Array(y)).StoreArray(dst)
	}
}

//go:noinline
func llvmSIMDMidwayAdd(x, y simd.Int8s) simd.Int8s {
	return x.Add(y)
}

// llvmSIMDMidwayGuardedAVX2 keeps its external signature independent of the
// portable vector width, so Midway generates a dispatcher and width variants.
// The fixed-width AVX2 operation is legal in the narrower variants only on the
// explicitly guarded path; their fallback remains baseline-safe.
//
//go:noinline
func llvmSIMDMidwayGuardedAVX2(dst, x, y *[32]int8) {
	var portable simd.Int8s
	_ = portable
	if archsimd.X86.AVX2() {
		archsimd.LoadInt8x32Array(x).Add(archsimd.LoadInt8x32Array(y)).StoreArray(dst)
		return
	}
	for i := range dst {
		dst[i] = x[i] + y[i]
	}
}
