// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && amd64

package codegen

import "simd/archsimd"

// Fixed 256-bit values only establish the AVX ABI floor. These operations
// require AVX2, so their matching feature guards produce baseline, AVX2, and
// resolver variants. Fixed 512-bit values already establish the AVX512 floor.
// LLVM-AMD64-DAG: load i8, ptr getelementptr {{.*}}!goallc.cpu.guard !{{[0-9]+}}
// LLVM-AMD64-DAG: "goallc.cpu.multiversion"="x86.avx2"
// LLVM-AMD64-DAG: !{!"x86.avx2"}
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmSIMDComposeAverageUnsigned256<goallc.fmv.baseline>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmSIMDComposeAverageUnsigned256<goallc.fmv.avx2>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmSIMDComposeAverageUnsigned256<goallc.fmv.resolve>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmSIMDComposeMulHighSigned256<goallc.fmv.avx2>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmSIMDComposeMulSign256<goallc.fmv.avx2>"
// LLVM-OPT-AMD64-DAG: "target-features"="+avx,+avx2"
// LLVM-NM-AMD64: codegen.llvmSIMDComposeAverageUnsigned256.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-3: codegen.llvmSIMDComposeAverageUnsigned256<1>
// LLVM-NM-AMD64: codegen.llvmSIMDComposeMulHighSigned256.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-3: codegen.llvmSIMDComposeMulHighSigned256<1>
// LLVM-NM-AMD64: codegen.llvmSIMDComposeMulSign256.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-3: codegen.llvmSIMDComposeMulSign256<1>

// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmSIMDComposeMulHighSigned(
// LLVM-AMD64-DAG: sext <8 x i16>
// LLVM-AMD64-DAG: mul <8 x i32>
// LLVM-AMD64-DAG: ashr <8 x i32>
// LLVM-AMD64-DAG: trunc <8 x i32>
// LLVM-AMD64-DAG: "goallc.cpu.feature-floor"="x86.avx"
// LLVM-ASM-AMD64-LABEL: TEXT codegen.llvmSIMDComposeMulHighSigned(SB)
// LLVM-ASM-AMD64: VPMULHW
//
//go:noinline
func llvmSIMDComposeMulHighSigned(x, y archsimd.Int16x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX() {
		return x
	}
	return x.MulHigh(y)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmSIMDComposeMulHighUnsigned(
// LLVM-AMD64-DAG: zext <8 x i16>
// LLVM-AMD64-DAG: mul <8 x i32>
// LLVM-AMD64-DAG: lshr <8 x i32>
// LLVM-AMD64-DAG: trunc <8 x i32>
// LLVM-ASM-AMD64-LABEL: TEXT codegen.llvmSIMDComposeMulHighUnsigned(SB)
// LLVM-ASM-AMD64: VPMULHUW
//
//go:noinline
func llvmSIMDComposeMulHighUnsigned(x, y archsimd.Uint16x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX() {
		return x
	}
	return x.MulHigh(y)
}

// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmSIMDComposeMulSign(
// LLVM-AMD64-DAG: call <16 x i8> @llvm.x86.ssse3.psign.b.128
// LLVM-ASM-AMD64-LABEL: TEXT codegen.llvmSIMDComposeMulSign(SB)
// LLVM-ASM-AMD64: VPSIGNB
//
//go:noinline
func llvmSIMDComposeMulSign(x, y archsimd.Int8x16) archsimd.Int8x16 {
	if !archsimd.X86.AVX() {
		return x
	}
	return x.MulSign(y)
}

// LLVM-AMD64-DAG: define {{.*}} <32 x i8> @codegen.llvmSIMDComposeAverageUnsigned256(
// LLVM-AMD64-DAG: xor <32 x i8>
// LLVM-AMD64-DAG: lshr <32 x i8>
// LLVM-AMD64-DAG: or <32 x i8>
// LLVM-AMD64-DAG: sub <32 x i8> {{.*}}!goallc.cpu.requires !{{[0-9]+}}
//
//go:noinline
func llvmSIMDComposeAverageUnsigned256(x, y archsimd.Uint8x32) archsimd.Uint8x32 {
	if !archsimd.X86.AVX2() {
		return x
	}
	return x.Average(y)
}

// LLVM-AMD64-DAG: define {{.*}} <64 x i8> @codegen.llvmSIMDComposeAverageUnsigned512(
// LLVM-AMD64-DAG: xor <64 x i8>
// LLVM-AMD64-DAG: lshr <64 x i8>
// LLVM-AMD64-DAG: or <64 x i8>
// LLVM-AMD64-DAG: sub <64 x i8>
// LLVM-AMD64-DAG: "goallc.cpu.feature-floor"="x86.avx512"
// LLVM-ASM-AMD64-LABEL: TEXT codegen.llvmSIMDComposeAverageUnsigned512(SB)
// LLVM-ASM-AMD64: VPAVGB
//
//go:noinline
func llvmSIMDComposeAverageUnsigned512(x, y archsimd.Uint8x64) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512() {
		return x
	}
	return x.Average(y)
}

// LLVM-AMD64-DAG: define {{.*}} <16 x i16> @codegen.llvmSIMDComposeMulHighSigned256(
// LLVM-AMD64-DAG: sext <16 x i16>
// LLVM-AMD64-DAG: mul <16 x i32>
// LLVM-AMD64-DAG: ashr <16 x i32>
// LLVM-AMD64-DAG: trunc <16 x i32> {{.*}}!goallc.cpu.requires !{{[0-9]+}}
//
//go:noinline
func llvmSIMDComposeMulHighSigned256(x, y archsimd.Int16x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return x
	}
	return x.MulHigh(y)
}

// LLVM-AMD64-DAG: define {{.*}} <32 x i16> @codegen.llvmSIMDComposeMulHighUnsigned512(
// LLVM-AMD64-DAG: zext <32 x i16>
// LLVM-AMD64-DAG: mul <32 x i32>
// LLVM-AMD64-DAG: lshr <32 x i32>
// LLVM-AMD64-DAG: trunc <32 x i32>
// LLVM-ASM-AMD64-LABEL: TEXT codegen.llvmSIMDComposeMulHighUnsigned512(SB)
// LLVM-ASM-AMD64: VPMULHUW
//
//go:noinline
func llvmSIMDComposeMulHighUnsigned512(x, y archsimd.Uint16x32) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return x
	}
	return x.MulHigh(y)
}

// LLVM-AMD64-DAG: define {{.*}} <32 x i8> @codegen.llvmSIMDComposeMulSign256(
// LLVM-AMD64-DAG: call <32 x i8> @llvm.x86.avx2.psign.b{{.*}}!goallc.cpu.requires !{{[0-9]+}}
//
//go:noinline
func llvmSIMDComposeMulSign256(x, y archsimd.Int8x32) archsimd.Int8x32 {
	if !archsimd.X86.AVX2() {
		return x
	}
	return x.MulSign(y)
}

// The optimized 256-bit instructions live in FMV implementation symbols, not
// in the public dispatchers above.
// LLVM-ASM-AMD64-DAG: VPAVGB Y
// LLVM-ASM-AMD64-DAG: VPMULHW Y
// LLVM-ASM-AMD64-DAG: VPSIGNB Y
