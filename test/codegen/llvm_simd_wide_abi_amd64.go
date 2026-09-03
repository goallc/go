// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && amd64

package codegen

import "simd/archsimd"

// A wide-vector callee establishes its own ABI floor. A scalar-signature
// caller under an explicit guard is the LLVM FMV boundary: the call, rather
// than every caller above it, carries the profile requirement.

// LLVM-AMD64-DAG: define goabiinternal <32 x i8> @codegen.llvmSIMDWideIdentity256(<32 x i8> %x) #[[IDENTITY256:[0-9]+]]
// LLVM-AMD64-DAG: define goabiinternal void @codegen.llvmSIMDGuardedWideCall256({{.*}}) #[[GUARDED256:[0-9]+]]
// LLVM-AMD64-DAG: call goabiinternal <32 x i8> @codegen.llvmSIMDWideIdentity256(<32 x i8> {{.*}}){{.*}}!goallc.cpu.requires ![[CALL_AVX2:[0-9]+]]
// LLVM-AMD64-DAG: load i8, ptr getelementptr {{.*}}!goallc.cpu.guard ![[CALL_AVX2]]
// LLVM-AMD64-DAG: ![[CALL_AVX2]] = !{!"x86.avx2"}
// LLVM-AMD64-DAG: attributes #[[IDENTITY256]] = { {{.*}}"goallc.cpu.feature-floor"="x86.avx"
// LLVM-AMD64-DAG: attributes #[[GUARDED256]] = { {{.*}}"goallc.cpu.multiversion"="x86.avx2"
// LLVM-NM-AMD64: codegen.llvmSIMDGuardedWideCall256.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-3: codegen.llvmSIMDGuardedWideCall256<1>
// LLVM-NM-AMD64-NOT: codegen.llvmSIMDGuardedWideCall256<1>
// LLVM-AMD64-DAG: define goabiinternal <64 x i8> @codegen.llvmSIMDWideIdentity512(<64 x i8> %x) #[[IDENTITY512:[0-9]+]]
// LLVM-AMD64-DAG: define goabiinternal void @codegen.llvmSIMDGuardedWideCall512({{.*}}) #[[GUARDED512:[0-9]+]]
// LLVM-AMD64-DAG: call goabiinternal <64 x i8> @codegen.llvmSIMDWideIdentity512(<64 x i8> {{.*}}){{.*}}!goallc.cpu.requires ![[CALL_AVX512:[0-9]+]]
// LLVM-AMD64-DAG: load i8, ptr getelementptr {{.*}}!goallc.cpu.guard ![[CALL_AVX512]]
// LLVM-AMD64-DAG: ![[CALL_AVX512]] = !{!"x86.avx512"}
// LLVM-AMD64-DAG: attributes #[[IDENTITY512]] = { {{.*}}"goallc.cpu.feature-floor"="x86.avx512"
// LLVM-AMD64-DAG: attributes #[[GUARDED512]] = { {{.*}}"goallc.cpu.multiversion"="x86.avx512"
// LLVM-NM-AMD64: codegen.llvmSIMDGuardedWideCall512.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-3: codegen.llvmSIMDGuardedWideCall512<1>
// LLVM-NM-AMD64-NOT: codegen.llvmSIMDGuardedWideCall512<1>
//
//go:noinline
func llvmSIMDWideIdentity256(x archsimd.Uint8x32) archsimd.Uint8x32 {
	return x
}

//go:noinline
func llvmSIMDGuardedWideCall256(dst, src *[32]uint8) {
	if !archsimd.X86.AVX2() {
		for i := range dst {
			dst[i] = src[i]
		}
		return
	}
	llvmSIMDWideIdentity256(archsimd.LoadUint8x32Array(src)).StoreArray(dst)
}

//go:noinline
func llvmSIMDWideIdentity512(x archsimd.Uint8x64) archsimd.Uint8x64 {
	return x
}

//go:noinline
func llvmSIMDGuardedWideCall512(dst, src *[64]uint8) {
	if !archsimd.X86.AVX512() {
		for i := range dst {
			dst[i] = src[i]
		}
		return
	}
	llvmSIMDWideIdentity512(archsimd.LoadUint8x64Array(src)).StoreArray(dst)
}

// An unguarded fixed-width call retains native Go's contract: the caller gets
// a function floor instead of an implicit dispatcher.

// LLVM-AMD64-DAG: define goabiinternal void @codegen.llvmSIMDUnguardedWideCall256({{.*}}) #[[IDENTITY256]]
// LLVM-NM-AMD64-NOT: codegen.llvmSIMDUnguardedWideCall256.goallc.fmv.slot
//
//go:noinline
func llvmSIMDUnguardedWideCall256(dst, src *[32]uint8) {
	llvmSIMDWideIdentity256(archsimd.LoadUint8x32Array(src)).StoreArray(dst)
}
