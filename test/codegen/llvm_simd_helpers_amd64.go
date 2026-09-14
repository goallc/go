// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-AMD64-DAG: trunc i8 {{.*}} to i4
// LLVM-AMD64-DAG: bitcast i4 {{.*}} to <4 x i1>
// LLVM-AMD64-DAG: sext <4 x i1> {{.*}} to <4 x i32>
// LLVM-OPT-AMD64-DAG: @"codegen.maskFromBits<goallc.fmv.avx512>"
//
//go:noinline
func maskFromBits(bits uint8, out *[4]int32) {
	if archsimd.X86.AVX512() {
		archsimd.Mask32x4FromBits(bits).ToInt32x4().StoreArray(out)
	}
}

// LLVM-AMD64-DAG: icmp slt <4 x i32>
// LLVM-AMD64-DAG: bitcast <4 x i1> {{.*}} to i4
// LLVM-AMD64-DAG: zext i4 {{.*}} to i8
//
//go:noinline
func maskToBits(mask archsimd.Mask32x4) uint8 {
	return mask.ToBits()
}

// LLVM-AMD64-DAG: call i8 @llvm.vector.reduce.or.v32i8
//
//go:noinline
func maskIsZero(x archsimd.Uint8x32) bool {
	return x.IsZero()
}

// LLVM-AMD64-DAG: fcmp uno <4 x float>
//
//go:noinline
func maskIsNaN(x archsimd.Float32x4) archsimd.Mask32x4 {
	return x.IsNaN()
}

// LLVM-AMD64-DAG: call void @llvm.masked.store.v4i32.p0(<4 x i32> {{.*}}, ptr {{.*}}, <4 x i1> {{.*}})
// LLVM-OPT-AMD64-DAG: @"codegen.maskStore<goallc.fmv.avx2>"
//
//go:noinline
func maskStore(x, mask archsimd.Int32x4, out *[4]int32) {
	if archsimd.X86.AVX2() {
		x.StoreArrayMasked(out, mask.Less(archsimd.Int32x4{}))
	}
}

// Folding the constructed mask must not remove its stronger Go precondition.
// LLVM-AMD64-DAG: call void @llvm.sideeffect(){{.*}}!goallc.cpu.requires ![[MASKREQ:[0-9]+]]
// LLVM-AMD64-DAG: ![[MASKREQ]] = !{!"x86.avx512"}
// LLVM-OPT-AMD64-DAG: @"codegen.maskZeroBits<goallc.fmv.avx512>"
//
//go:noinline
func maskZeroBits(out *[4]int32) {
	if archsimd.X86.AVX512() {
		archsimd.Mask32x4FromBits(0).ToInt32x4().StoreArray(out)
	}
}
