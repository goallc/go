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

// This fixture makes the runtime example's dispatch matrix inspectable before
// and after the early LLVM FMV pass. Each Midway floor removes only the CPU
// profiles whose code-generation requirements it already covers.

// LLVM-AMD64-DAG: define goabiinternal void @"codegen.llvmSIMDDispatchFMVMatrix@simd0"{{.*}} #[[MATRIX0:[0-9]+]]
// LLVM-AMD64-DAG: define goabiinternal void @"codegen.llvmSIMDDispatchFMVMatrix@simd128"{{.*}} #[[MATRIX128:[0-9]+]]
// LLVM-AMD64-DAG: define goabiinternal void @"codegen.llvmSIMDDispatchFMVMatrix@simd256"{{.*}} #[[MATRIX256:[0-9]+]]
// LLVM-AMD64-DAG: define goabiinternal void @"codegen.llvmSIMDDispatchFMVMatrix@simd512"{{.*}} #[[MATRIX512:[0-9]+]]
// LLVM-AMD64-DAG: attributes #[[MATRIX0]] = { {{.*}}"goallc.cpu.multiversion"="x86.avx2,x86.avx512"
// LLVM-AMD64-DAG: attributes #[[MATRIX128]] = { {{.*}}"goallc.cpu.feature-floor"="x86.avx"{{.*}}"goallc.cpu.multiversion"="x86.avx2,x86.avx512"
// LLVM-AMD64-DAG: attributes #[[MATRIX256]] = { {{.*}}"goallc.cpu.feature-floor"="x86.avx2"{{.*}}"goallc.cpu.multiversion"="x86.avx512"
// LLVM-AMD64-DAG: attributes #[[MATRIX512]] = { {{.*}}"goallc.cpu.feature-floor"="x86.avx512"
// LLVM-NM-AMD64: codegen.llvmSIMDDispatchFMVMatrix@simd0.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-5: codegen.llvmSIMDDispatchFMVMatrix@simd0<1>
// LLVM-NM-AMD64-NOT: codegen.llvmSIMDDispatchFMVMatrix@simd0<1>
// LLVM-NM-AMD64: codegen.llvmSIMDDispatchFMVMatrix@simd128.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-5: codegen.llvmSIMDDispatchFMVMatrix@simd128<1>
// LLVM-NM-AMD64-NOT: codegen.llvmSIMDDispatchFMVMatrix@simd128<1>
// LLVM-NM-AMD64: codegen.llvmSIMDDispatchFMVMatrix@simd256.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-3: codegen.llvmSIMDDispatchFMVMatrix@simd256<1>
// LLVM-NM-AMD64-NOT: codegen.llvmSIMDDispatchFMVMatrix@simd256<1>
// LLVM-NM-AMD64-NOT: codegen.llvmSIMDDispatchFMVMatrix@simd512.goallc.fmv.slot
// LLVM-NM-AMD64-NOT: codegen.llvmSIMDDispatchFMVMatrix@simd512<1>

//go:noinline
func llvmSIMDDispatchFMVIdentity256ForMatrix(x archsimd.Int8x32) archsimd.Int8x32 {
	return x
}

//go:noinline
func llvmSIMDDispatchFMVIdentity512ForMatrix(x archsimd.Int8x64) archsimd.Int8x64 {
	return x
}

//go:noinline
func llvmSIMDDispatchFMVMatrix(
	portableDst *[64]int8,
	wide256Dst *[32]int8,
	wide512Dst *[64]int8,
	x, y *[64]int8,
	x256, y256 *[32]int8,
) {
	simd.LoadInt8s(x[:]).Add(simd.LoadInt8s(y[:])).Store(portableDst[:])

	if archsimd.X86.AVX2() {
		value := archsimd.LoadInt8x32Array(x256).Add(archsimd.LoadInt8x32Array(y256))
		llvmSIMDDispatchFMVIdentity256ForMatrix(value).StoreArray(wide256Dst)
	} else {
		for i := range wide256Dst {
			wide256Dst[i] = x256[i] + y256[i]
		}
	}

	if archsimd.X86.AVX512() {
		value := archsimd.LoadInt8x64Array(x).Add(archsimd.LoadInt8x64Array(y))
		llvmSIMDDispatchFMVIdentity512ForMatrix(value).StoreArray(wide512Dst)
	} else {
		for i := range wide512Dst {
			wide512Dst[i] = x[i] + y[i]
		}
	}
}
