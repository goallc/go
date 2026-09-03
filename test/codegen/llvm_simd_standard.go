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

// LLVM-AMD64-DAG: call <4 x float> @llvm.ceil.v4f32
// LLVM-AMD64-DAG: call <4 x float> @llvm.floor.v4f32
// LLVM-AMD64-DAG: call <4 x float> @llvm.roundeven.v4f32
// LLVM-AMD64-DAG: call <4 x float> @llvm.sqrt.v4f32
// LLVM-AMD64-DAG: call <4 x float> @llvm.trunc.v4f32
// LLVM-ARM64-DAG: call <4 x float> @llvm.ceil.v4f32
// LLVM-ARM64-DAG: call <4 x float> @llvm.floor.v4f32
// LLVM-ARM64-DAG: call <4 x float> @llvm.roundeven.v4f32
// LLVM-ARM64-DAG: call <4 x float> @llvm.sqrt.v4f32
// LLVM-ARM64-DAG: call <4 x float> @llvm.trunc.v4f32
//
//go:noinline
func llvmSIMDStandardFloatUnary(x archsimd.Float32x4) (archsimd.Float32x4, archsimd.Float32x4, archsimd.Float32x4, archsimd.Float32x4, archsimd.Float32x4) {
	return x.Ceil(), x.Floor(), x.Round(), x.Sqrt(), x.Trunc()
}

// LLVM-AMD64-DAG: call <8 x i16> @llvm.smin.v8i16
// LLVM-AMD64-DAG: call <8 x i16> @llvm.smax.v8i16
// LLVM-ARM64-DAG: call <8 x i16> @llvm.smin.v8i16
// LLVM-ARM64-DAG: call <8 x i16> @llvm.smax.v8i16
//
//go:noinline
func llvmSIMDStandardSignedMinMax(x, y archsimd.Int16x8) (archsimd.Int16x8, archsimd.Int16x8) {
	return x.Min(y), x.Max(y)
}

// LLVM-AMD64-DAG: call <4 x i32> @llvm.umin.v4i32
// LLVM-AMD64-DAG: call <4 x i32> @llvm.umax.v4i32
// LLVM-ARM64-DAG: call <4 x i32> @llvm.umin.v4i32
// LLVM-ARM64-DAG: call <4 x i32> @llvm.umax.v4i32
//
//go:noinline
func llvmSIMDStandardUnsignedMinMax(x, y archsimd.Uint32x4) (archsimd.Uint32x4, archsimd.Uint32x4) {
	return x.Min(y), x.Max(y)
}

// LLVM-AMD64-DAG: call <4 x float> @llvm.minimum.v4f32
// LLVM-AMD64-DAG: call <4 x float> @llvm.maximum.v4f32
// LLVM-ARM64-DAG: call <4 x float> @llvm.minimum.v4f32
// LLVM-ARM64-DAG: call <4 x float> @llvm.maximum.v4f32
//
//go:noinline
func llvmSIMDStandardFloatMinMax(x, y archsimd.Float32x4) (archsimd.Float32x4, archsimd.Float32x4) {
	return x.Min(y), x.Max(y)
}

// LLVM-AMD64-DAG: call <4 x i32> @llvm.ctlz.v4i32(<4 x i32> {{.*}}, i1 false)
// LLVM-ARM64-DAG: call <4 x i32> @llvm.ctlz.v4i32(<4 x i32> {{.*}}, i1 false)
//
//go:noinline
func llvmSIMDStandardLeadingZeros(x archsimd.Uint32x4) archsimd.Uint32x4 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return x
	}
	return x.LeadingZeros()
}

// LLVM-AMD64-DAG: load i8, ptr getelementptr {{.*}}!goallc.cpu.guard ![[BITALG:[0-9]+]]
// LLVM-AMD64-DAG: call <16 x i8> @llvm.ctpop.v16i8{{.*}}!goallc.cpu.requires ![[BITALG]]
// LLVM-AMD64-DAG: ![[BITALG]] = !{!"x86.avx512bitalg"}
// LLVM-ARM64-DAG: call <16 x i8> @llvm.ctpop.v16i8
//
//go:noinline
func llvmSIMDStandardOnesCount(x archsimd.Uint8x16) archsimd.Uint8x16 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512BITALG() {
		return x
	}
	return x.OnesCount()
}
