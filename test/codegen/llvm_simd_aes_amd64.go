// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-AMD64-DAG: call <2 x i64> @llvm.x86.aesni.aesenc(
// LLVM-AMD64-DAG: call <2 x i64> @llvm.x86.aesni.aesenclast(
// LLVM-AMD64-DAG: call <2 x i64> @llvm.x86.aesni.aesdec(
// LLVM-AMD64-DAG: call <2 x i64> @llvm.x86.aesni.aesdeclast(
// LLVM-AMD64-DAG: !{!"x86.avxaes"}
// LLVM-ASM-AMD64-DAG: {{V?AESENC .*X[0-9]}}
//
//go:noinline
func aes128(x archsimd.Uint8x16, key archsimd.Uint32x4) archsimd.Uint8x16 {
	if !archsimd.X86.AVXAES() {
		return x
	}
	x = x.AESEncryptOneRound(key)
	x = x.AESEncryptLastRound(key)
	x = x.AESDecryptOneRound(key)
	return x.AESDecryptLastRound(key)
}

// LLVM-AMD64-DAG: call <4 x i64> @llvm.x86.aesni.aesenc.256(
// LLVM-AMD64-DAG: call <4 x i64> @llvm.x86.aesni.aesenclast.256(
// LLVM-AMD64-DAG: call <4 x i64> @llvm.x86.aesni.aesdec.256(
// LLVM-AMD64-DAG: call <4 x i64> @llvm.x86.aesni.aesdeclast.256(
// LLVM-AMD64-DAG: !{!"x86.vaes"}
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.aes256<goallc.fmv.vaes>"({{.*}}) [[ATTR256:#[0-9]+]]
// LLVM-OPT-AMD64-DAG: attributes [[ATTR256]] = { {{.*}}"target-features"="{{[^"]*}}+vaes{{[^"]*}}"
// LLVM-ASM-AMD64-DAG: VAESENC {{.*Y[0-9]}}
//
//go:noinline
func aes256(x archsimd.Uint8x32, key archsimd.Uint32x8) archsimd.Uint8x32 {
	if !archsimd.X86.VAES() {
		return x
	}
	x = x.AESEncryptOneRound(key)
	x = x.AESEncryptLastRound(key)
	x = x.AESDecryptOneRound(key)
	return x.AESDecryptLastRound(key)
}

// LLVM-AMD64-DAG: call <8 x i64> @llvm.x86.aesni.aesenc.512(
// LLVM-AMD64-DAG: call <8 x i64> @llvm.x86.aesni.aesenclast.512(
// LLVM-AMD64-DAG: call <8 x i64> @llvm.x86.aesni.aesdec.512(
// LLVM-AMD64-DAG: call <8 x i64> @llvm.x86.aesni.aesdeclast.512(
// LLVM-AMD64-DAG: !{!"x86.avx512vaes"}
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.aes512<goallc.fmv.avx512vaes>"({{.*}}) [[ATTR512:#[0-9]+]]
// LLVM-OPT-AMD64-DAG: attributes [[ATTR512]] = { {{.*}}"target-features"="{{[^"]*}}+avx512f{{[^"]*}}+vaes{{[^"]*}}"
// LLVM-ASM-AMD64-DAG: VAESENC {{.*Z[0-9]}}
//
//go:noinline
func aes512(x archsimd.Uint8x64, key archsimd.Uint32x16) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512VAES() {
		return x
	}
	x = x.AESEncryptOneRound(key)
	x = x.AESEncryptLastRound(key)
	x = x.AESDecryptOneRound(key)
	return x.AESDecryptLastRound(key)
}

// LLVM-AMD64-DAG: call <2 x i64> @llvm.x86.aesni.aeskeygenassist({{.*}}i8 -1)
// LLVM-AMD64-DAG: call <2 x i64> @llvm.x86.aesni.aesimc(
// LLVM-ASM-AMD64-DAG: AESKEYGENASSIST
// LLVM-ASM-AMD64-DAG: AESIMC
//
//go:noinline
func aesKey(x archsimd.Uint32x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVXAES() {
		return x
	}
	return x.AESRoundKeyGenAssist(0xff).AESInvMixColumns()
}
