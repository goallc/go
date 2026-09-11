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

// LLVM-AMD64-DAG: sext <8 x i8> {{.*}} to <8 x i16>

// LLVM-ARM64-DAG: sext <8 x i8> {{.*}} to <8 x i16>
//
//go:noinline
func llvmExtendLo8ToInt16Int8x16(x archsimd.Int8x16) archsimd.Int16x8 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return archsimd.Int16x8{}
	}
	return x.ExtendLo8ToInt16()
}

// LLVM-AMD64-DAG: zext <8 x i8> {{.*}} to <8 x i16>

// LLVM-ARM64-DAG: zext <8 x i8> {{.*}} to <8 x i16>
//
//go:noinline
func llvmExtendLo8ToUint16Uint8x16(x archsimd.Uint8x16) archsimd.Uint16x8 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return archsimd.Uint16x8{}
	}
	return x.ExtendLo8ToUint16()
}

// LLVM-AMD64-DAG: trunc <8 x i16> {{.*}} to <8 x i8>

// LLVM-ARM64-DAG: trunc <8 x i16> {{.*}} to <8 x i8>
// LLVM-AMD64-DAG: shufflevector <8 x i8> {{.*}}, <8 x i8> zeroinitializer
//
//go:noinline
func llvmTruncToInt8Int16x8(x archsimd.Int16x8) archsimd.Int8x16 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Int8x16{}
	}
	return x.TruncToInt8()
}
