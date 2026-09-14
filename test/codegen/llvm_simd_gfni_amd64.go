// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-AMD64-DAG: call <16 x i8> @llvm.x86.vgf2p8affineqb.128({{.*}}i8 -1)
// LLVM-AMD64-DAG: call <16 x i8> @llvm.x86.vgf2p8affineinvqb.128({{.*}}i8 99)
// LLVM-AMD64-DAG: call <16 x i8> @llvm.x86.vgf2p8mulb.128(
// LLVM-AMD64-DAG: !{!"x86.avx512gfni"}
// LLVM-ASM-AMD64-DAG: VGF2P8AFFINEQB {{.*X[0-9]}}
// LLVM-ASM-AMD64-DAG: VGF2P8AFFINEINVQB {{.*X[0-9]}}
// LLVM-ASM-AMD64-DAG: VGF2P8MULB {{.*X[0-9]}}
//
//go:noinline
func gf128(x, y archsimd.Uint8x16, matrix archsimd.Uint64x2) archsimd.Uint8x16 {
	if !archsimd.X86.AVX512GFNI() {
		return x
	}
	x = x.GaloisFieldAffineTransform(matrix, 0xff)
	x = x.GaloisFieldAffineTransformInverse(matrix, 0x63)
	return x.GaloisFieldMul(y)
}

// LLVM-AMD64-DAG: call <32 x i8> @llvm.x86.vgf2p8affineqb.256({{.*}}i8 -1)
// LLVM-AMD64-DAG: call <32 x i8> @llvm.x86.vgf2p8affineinvqb.256({{.*}}i8 99)
// LLVM-AMD64-DAG: call <32 x i8> @llvm.x86.vgf2p8mulb.256(
// LLVM-ASM-AMD64-DAG: VGF2P8AFFINEQB {{.*Y[0-9]}}
// LLVM-ASM-AMD64-DAG: VGF2P8AFFINEINVQB {{.*Y[0-9]}}
// LLVM-ASM-AMD64-DAG: VGF2P8MULB {{.*Y[0-9]}}
//
//go:noinline
func gf256(x, y archsimd.Uint8x32, matrix archsimd.Uint64x4) archsimd.Uint8x32 {
	if !archsimd.X86.AVX512GFNI() {
		return x
	}
	x = x.GaloisFieldAffineTransform(matrix, 0xff)
	x = x.GaloisFieldAffineTransformInverse(matrix, 0x63)
	return x.GaloisFieldMul(y)
}

// LLVM-AMD64-DAG: call <64 x i8> @llvm.x86.vgf2p8affineqb.512({{.*}}i8 -1)
// LLVM-AMD64-DAG: call <64 x i8> @llvm.x86.vgf2p8affineinvqb.512({{.*}}i8 99)
// LLVM-AMD64-DAG: call <64 x i8> @llvm.x86.vgf2p8mulb.512(
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.gf512<goallc.fmv.avx512gfni>"({{.*}}) [[GFATTR:#[0-9]+]]
// LLVM-OPT-AMD64-DAG: attributes [[GFATTR]] = { {{.*}}"target-features"="{{[^"]*}}+avx512f{{[^"]*}}+gfni{{[^"]*}}"
// LLVM-ASM-AMD64-DAG: VGF2P8AFFINEQB {{.*Z[0-9]}}
// LLVM-ASM-AMD64-DAG: VGF2P8AFFINEINVQB {{.*Z[0-9]}}
// LLVM-ASM-AMD64-DAG: VGF2P8MULB {{.*Z[0-9]}}
//
//go:noinline
func gf512(x, y archsimd.Uint8x64, matrix archsimd.Uint64x8) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512GFNI() {
		return x
	}
	x = x.GaloisFieldAffineTransform(matrix, 0xff)
	x = x.GaloisFieldAffineTransformInverse(matrix, 0x63)
	return x.GaloisFieldMul(y)
}
