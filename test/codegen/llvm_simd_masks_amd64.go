// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-AMD64-DAG: call <4 x float> @llvm.x86.avx512.mask.compress.v4f32(<4 x float> {{.*}}, <4 x float> zeroinitializer, <4 x i1> {{.*}})
// LLVM-AMD64-DAG: call <4 x float> @llvm.x86.avx512.mask.expand.v4f32(<4 x float> {{.*}}, <4 x float> zeroinitializer, <4 x i1> {{.*}})
// LLVM-ASM-AMD64-DAG: VCOMPRESSPS{{.*X[0-9]}}
// LLVM-ASM-AMD64-DAG: VEXPANDPS{{.*X[0-9]}}
//
//go:noinline
func masksFloat32x4(x archsimd.Float32x4, mask archsimd.Mask32x4) (archsimd.Float32x4, archsimd.Float32x4) {
	if archsimd.X86.AVX512() {
		return x.Compress(mask), x.Expand(mask)
	}
	return x, x
}

// LLVM-AMD64-DAG: call <4 x double> @llvm.x86.avx512.mask.compress.v4f64(<4 x double> {{.*}}, <4 x double> zeroinitializer, <4 x i1> {{.*}})
// LLVM-AMD64-DAG: call <4 x double> @llvm.x86.avx512.mask.expand.v4f64(<4 x double> {{.*}}, <4 x double> zeroinitializer, <4 x i1> {{.*}})
// LLVM-ASM-AMD64-DAG: VCOMPRESSPD{{.*Y[0-9]}}
// LLVM-ASM-AMD64-DAG: VEXPANDPD{{.*Y[0-9]}}
//
//go:noinline
func masksFloat64x4(x archsimd.Float64x4, mask archsimd.Mask64x4) (archsimd.Float64x4, archsimd.Float64x4) {
	if archsimd.X86.AVX512() {
		return x.Compress(mask), x.Expand(mask)
	}
	return x, x
}

// LLVM-AMD64-DAG: call <64 x i8> @llvm.x86.avx512.mask.compress.v64i8(<64 x i8> {{.*}}, <64 x i8> zeroinitializer, <64 x i1> {{.*}})
// LLVM-AMD64-DAG: call <64 x i8> @llvm.x86.avx512.mask.expand.v64i8(<64 x i8> {{.*}}, <64 x i8> zeroinitializer, <64 x i1> {{.*}})
// LLVM-ASM-AMD64-DAG: VPCOMPRESSB{{.*Z[0-9]}}
// LLVM-ASM-AMD64-DAG: VPEXPANDB{{.*Z[0-9]}}
//
//go:noinline
func masksInt8x64(x archsimd.Int8x64, mask archsimd.Mask8x64) (archsimd.Int8x64, archsimd.Int8x64) {
	if archsimd.X86.AVX512VBMI2() {
		return x.Compress(mask), x.Expand(mask)
	}
	return x, x
}

// LLVM-AMD64-DAG: call <8 x i16> @llvm.x86.avx512.mask.compress.v8i16(<8 x i16> {{.*}}, <8 x i16> zeroinitializer, <8 x i1> {{.*}})
// LLVM-AMD64-DAG: call <8 x i16> @llvm.x86.avx512.mask.expand.v8i16(<8 x i16> {{.*}}, <8 x i16> zeroinitializer, <8 x i1> {{.*}})
// LLVM-ASM-AMD64-DAG: VPCOMPRESSW{{.*X[0-9]}}
// LLVM-ASM-AMD64-DAG: VPEXPANDW{{.*X[0-9]}}
//
//go:noinline
func masksInt16x8(x archsimd.Int16x8, mask archsimd.Mask16x8) (archsimd.Int16x8, archsimd.Int16x8) {
	if archsimd.X86.AVX512VBMI2() {
		return x.Compress(mask), x.Expand(mask)
	}
	return x, x
}

// LLVM-AMD64-DAG: call <16 x i32> @llvm.x86.avx512.mask.compress.v16i32(<16 x i32> {{.*}}, <16 x i32> zeroinitializer, <16 x i1> {{.*}})
// LLVM-AMD64-DAG: call <16 x i32> @llvm.x86.avx512.mask.expand.v16i32(<16 x i32> {{.*}}, <16 x i32> zeroinitializer, <16 x i1> {{.*}})
// LLVM-ASM-AMD64-DAG: VPCOMPRESSD{{.*Z[0-9]}}
// LLVM-ASM-AMD64-DAG: VPEXPANDD{{.*Z[0-9]}}
//
//go:noinline
func masksInt32x16(x archsimd.Int32x16, mask archsimd.Mask32x16) (archsimd.Int32x16, archsimd.Int32x16) {
	if archsimd.X86.AVX512() {
		return x.Compress(mask), x.Expand(mask)
	}
	return x, x
}

// LLVM-AMD64-DAG: call <4 x i64> @llvm.x86.avx512.mask.compress.v4i64(<4 x i64> {{.*}}, <4 x i64> zeroinitializer, <4 x i1> {{.*}})
// LLVM-AMD64-DAG: call <4 x i64> @llvm.x86.avx512.mask.expand.v4i64(<4 x i64> {{.*}}, <4 x i64> zeroinitializer, <4 x i1> {{.*}})
// LLVM-ASM-AMD64-DAG: VPCOMPRESSQ{{.*Y[0-9]}}
// LLVM-ASM-AMD64-DAG: VPEXPANDQ{{.*Y[0-9]}}
//
//go:noinline
func masksInt64x4(x archsimd.Int64x4, mask archsimd.Mask64x4) (archsimd.Int64x4, archsimd.Int64x4) {
	if archsimd.X86.AVX512() {
		return x.Compress(mask), x.Expand(mask)
	}
	return x, x
}

// LLVM-AMD64-DAG: !{!"x86.avx512vbmi2"}
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.masksInt8x64<goallc.fmv.avx512vbmi2>"({{.*}}) [[MASKATTR:#[0-9]+]]
// LLVM-OPT-AMD64-DAG: attributes [[MASKATTR]] = { {{.*}}"target-features"="{{[^"]*}}+avx512vbmi2{{[^"]*}}"
