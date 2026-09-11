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

// LLVM-ASM-AMD64-DAG: VPACKSSWB X1, X0, X0
// LLVM-ASM-AMD64-DAG: VPACKSSDW X1, X0, X0
// LLVM-ASM-AMD64-DAG: VPMOVSQD X0, X0
// LLVM-ASM-AMD64-DAG: VPMOVUSWB X0, X0
// LLVM-ASM-AMD64-DAG: VPMOVUSDW X0, X0
// LLVM-ASM-AMD64-DAG: VPMOVUSQD X0, X0
// LLVM-ASM-ARM64-DAG: VSQXTN V0.H8, V0.B8
// LLVM-ASM-ARM64-DAG: VSQXTN V0.S4, V0.H4
// LLVM-ASM-ARM64-DAG: VSQXTN V0.D2, V0.S2
// LLVM-ASM-ARM64-DAG: VUQXTN V0.H8, V0.B8
// LLVM-ASM-ARM64-DAG: VUQXTN V0.S4, V0.H4
// LLVM-ASM-ARM64-DAG: VUQXTN V0.D2, V0.S2

// LLVM-AMD64-DAG: call <8 x i16> @llvm.smax.v8i16({{.*}}i16 -128
// LLVM-AMD64-DAG: call <8 x i16> @llvm.smin.v8i16({{.*}}i16 127
// LLVM-AMD64-DAG: trunc <8 x i16> {{.*}} to <8 x i8>
// LLVM-AMD64-DAG: shufflevector <8 x i8> {{.*}}, <8 x i8> zeroinitializer
// LLVM-ARM64-DAG: call <8 x i16> @llvm.smax.v8i16({{.*}}i16 -128
// LLVM-ARM64-DAG: call <8 x i16> @llvm.smin.v8i16({{.*}}i16 127
// LLVM-ARM64-DAG: trunc <8 x i16> {{.*}} to <8 x i8>
// LLVM-ARM64-DAG: shufflevector <8 x i8> {{.*}}, <8 x i8> zeroinitializer
//
//go:noinline
func llvmSaturateToInt8Int16x8(x archsimd.Int16x8) archsimd.Int8x16 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Int8x16{}
	}
	return x.SaturateToInt8()
}

// LLVM-AMD64-DAG: call <4 x i32> @llvm.smax.v4i32({{.*}}i32 -32768
// LLVM-AMD64-DAG: call <4 x i32> @llvm.smin.v4i32({{.*}}i32 32767
// LLVM-AMD64-DAG: trunc <4 x i32> {{.*}} to <4 x i16>
// LLVM-AMD64-DAG: shufflevector <4 x i16> {{.*}}, <4 x i16> zeroinitializer
// LLVM-ARM64-DAG: call <4 x i32> @llvm.smax.v4i32({{.*}}i32 -32768
// LLVM-ARM64-DAG: call <4 x i32> @llvm.smin.v4i32({{.*}}i32 32767
// LLVM-ARM64-DAG: trunc <4 x i32> {{.*}} to <4 x i16>
// LLVM-ARM64-DAG: shufflevector <4 x i16> {{.*}}, <4 x i16> zeroinitializer
//
//go:noinline
func llvmSaturateToInt16Int32x4(x archsimd.Int32x4) archsimd.Int16x8 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Int16x8{}
	}
	return x.SaturateToInt16()
}

// LLVM-AMD64-DAG: call <2 x i64> @llvm.smax.v2i64({{.*}}i64 -2147483648
// LLVM-AMD64-DAG: call <2 x i64> @llvm.smin.v2i64({{.*}}i64 2147483647
// LLVM-AMD64-DAG: trunc <2 x i64> {{.*}} to <2 x i32>
// LLVM-AMD64-DAG: shufflevector <2 x i32> {{.*}}, <2 x i32> zeroinitializer
// LLVM-ARM64-DAG: call <2 x i64> @llvm.smax.v2i64({{.*}}i64 -2147483648
// LLVM-ARM64-DAG: call <2 x i64> @llvm.smin.v2i64({{.*}}i64 2147483647
// LLVM-ARM64-DAG: trunc <2 x i64> {{.*}} to <2 x i32>
// LLVM-ARM64-DAG: shufflevector <2 x i32> {{.*}}, <2 x i32> zeroinitializer
//
//go:noinline
func llvmSaturateToInt32Int64x2(x archsimd.Int64x2) archsimd.Int32x4 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Int32x4{}
	}
	return x.SaturateToInt32()
}

// LLVM-AMD64-DAG: call <8 x i16> @llvm.umin.v8i16({{.*}}i16 255
// LLVM-AMD64-DAG: trunc <8 x i16> {{.*}} to <8 x i8>
// LLVM-AMD64-DAG: shufflevector <8 x i8> {{.*}}, <8 x i8> zeroinitializer
// LLVM-ARM64-DAG: call <8 x i16> @llvm.umin.v8i16({{.*}}i16 255
// LLVM-ARM64-DAG: trunc <8 x i16> {{.*}} to <8 x i8>
// LLVM-ARM64-DAG: shufflevector <8 x i8> {{.*}}, <8 x i8> zeroinitializer
//
//go:noinline
func llvmSaturateToUint8Uint16x8(x archsimd.Uint16x8) archsimd.Uint8x16 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Uint8x16{}
	}
	return x.SaturateToUint8()
}

// LLVM-AMD64-DAG: call <4 x i32> @llvm.umin.v4i32({{.*}}i32 65535
// LLVM-AMD64-DAG: trunc <4 x i32> {{.*}} to <4 x i16>
// LLVM-AMD64-DAG: shufflevector <4 x i16> {{.*}}, <4 x i16> zeroinitializer
// LLVM-ARM64-DAG: call <4 x i32> @llvm.umin.v4i32({{.*}}i32 65535
// LLVM-ARM64-DAG: trunc <4 x i32> {{.*}} to <4 x i16>
// LLVM-ARM64-DAG: shufflevector <4 x i16> {{.*}}, <4 x i16> zeroinitializer
//
//go:noinline
func llvmSaturateToUint16Uint32x4(x archsimd.Uint32x4) archsimd.Uint16x8 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Uint16x8{}
	}
	return x.SaturateToUint16()
}

// LLVM-AMD64-DAG: call <2 x i64> @llvm.umin.v2i64({{.*}}i64 4294967295
// LLVM-AMD64-DAG: trunc <2 x i64> {{.*}} to <2 x i32>
// LLVM-AMD64-DAG: shufflevector <2 x i32> {{.*}}, <2 x i32> zeroinitializer
// LLVM-ARM64-DAG: call <2 x i64> @llvm.umin.v2i64({{.*}}i64 4294967295
// LLVM-ARM64-DAG: trunc <2 x i64> {{.*}} to <2 x i32>
// LLVM-ARM64-DAG: shufflevector <2 x i32> {{.*}}, <2 x i32> zeroinitializer
//
//go:noinline
func llvmSaturateToUint32Uint64x2(x archsimd.Uint64x2) archsimd.Uint32x4 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Uint32x4{}
	}
	return x.SaturateToUint32()
}
