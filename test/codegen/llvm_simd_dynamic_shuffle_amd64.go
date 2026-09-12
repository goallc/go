// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-AMD64-DAG: call <16 x i8> @llvm.x86.avx512.permvar.qi.128(
// LLVM-AMD64-DAG: call <16 x i8> @llvm.x86.avx512.vpermi2var.qi.128(
// LLVM-AMD64-DAG: call <32 x i8> @llvm.x86.avx512.permvar.qi.256(
// LLVM-AMD64-DAG: call <32 x i8> @llvm.x86.avx512.vpermi2var.qi.256(
// LLVM-AMD64-DAG: call <64 x i8> @llvm.x86.avx512.permvar.qi.512(
// LLVM-AMD64-DAG: call <64 x i8> @llvm.x86.avx512.vpermi2var.qi.512(
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmDynamicPermuteUint8x16<goallc.fmv.avx512vbmi>"
// LLVM-OPT-AMD64-DAG: and i64 %features, 8192
// LLVM-NM-AMD64-DAG: D codegen.llvmDynamicPermuteUint8x16.goallc.fmv.slot
// LLVM-ASM-AMD64-DAG: VPERM{{I2|T2}}B
// LLVM-ASM-AMD64-DAG: VPERMB
// LLVM-ASM-AMD64-DAG: VPERMW
// LLVM-ASM-AMD64-DAG: VPERM{{I2|T2}}W
// LLVM-ASM-AMD64-DAG: VPERM{{I2|T2}}D
// LLVM-ASM-AMD64-DAG: VPERM{{I2|T2}}Q
// LLVM-ASM-AMD64-DAG: VPERM{{D|PS}}
// LLVM-ASM-AMD64-DAG: VPERM{{Q|PD}}
// LLVM-ASM-AMD64-DAG: VPSHUFB

// LLVM-AMD64-DAG: call <8 x i32> @llvm.x86.avx2.permd(
// LLVM-AMD64-DAG: call <16 x i32> @llvm.x86.avx512.permvar.si.512(
// LLVM-AMD64-DAG: call <4 x i64> @llvm.x86.avx512.permvar.di.256(
// LLVM-AMD64-DAG: call <8 x i64> @llvm.x86.avx512.permvar.di.512(
// LLVM-AMD64-DAG: call <8 x i16> @llvm.x86.avx512.permvar.hi.128(
// LLVM-AMD64-DAG: call <16 x i16> @llvm.x86.avx512.permvar.hi.256(
// LLVM-AMD64-DAG: call <32 x i16> @llvm.x86.avx512.permvar.hi.512(
// LLVM-AMD64-DAG: call <16 x i8> @llvm.x86.ssse3.pshuf.b.128(
// LLVM-AMD64-DAG: call <32 x i8> @llvm.x86.avx2.pshuf.b(
// LLVM-AMD64-DAG: call <64 x i8> @llvm.x86.avx512.pshuf.b.512(
// LLVM-AMD64-DAG: call <8 x i16> @llvm.x86.avx512.vpermi2var.hi.128(
// LLVM-AMD64-DAG: call <16 x i16> @llvm.x86.avx512.vpermi2var.hi.256(
// LLVM-AMD64-DAG: call <32 x i16> @llvm.x86.avx512.vpermi2var.hi.512(
// LLVM-AMD64-DAG: call <4 x i32> @llvm.x86.avx512.vpermi2var.d.128(
// LLVM-AMD64-DAG: call <8 x i32> @llvm.x86.avx512.vpermi2var.d.256(
// LLVM-AMD64-DAG: call <16 x i32> @llvm.x86.avx512.vpermi2var.d.512(
// LLVM-AMD64-DAG: call <2 x i64> @llvm.x86.avx512.vpermi2var.q.128(
// LLVM-AMD64-DAG: call <4 x i64> @llvm.x86.avx512.vpermi2var.q.256(
// LLVM-AMD64-DAG: call <8 x i64> @llvm.x86.avx512.vpermi2var.q.512(

// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteInt8x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteUint8x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteInt8x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteUint8x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteInt8x64(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteUint8x64(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteInt16x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteUint16x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteInt16x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteUint16x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteInt16x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteUint16x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteFloat32x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteInt32x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteUint32x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteFloat32x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteInt32x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteUint32x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteFloat32x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteInt32x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteUint32x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteFloat64x2(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteInt64x2(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteUint64x2(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteFloat64x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteInt64x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteUint64x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteFloat64x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteInt64x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicConcatPermuteUint64x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteInt8x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteUint8x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteInt8x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteUint8x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteInt8x64(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteUint8x64(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteInt16x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteUint16x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteInt16x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteUint16x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteInt16x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteUint16x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteFloat32x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteInt32x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteUint32x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteFloat32x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteInt32x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteUint32x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteFloat64x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteInt64x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteUint64x4(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteFloat64x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteInt64x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteUint64x8(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteOrZeroInt8x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteOrZeroUint8x16(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteOrZeroGroupedInt8x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteOrZeroGroupedInt8x64(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteOrZeroGroupedUint8x32(
// LLVM-AMD64-DAG: define {{.*}} @codegen.llvmDynamicPermuteOrZeroGroupedUint8x64(
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmDynamicPermuteFloat32x8<goallc.fmv.baseline>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmDynamicPermuteFloat32x8<goallc.fmv.avx2>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmDynamicPermuteFloat32x8<goallc.fmv.resolve>"
// LLVM-NM-AMD64-DAG: D codegen.llvmDynamicPermuteFloat32x8.goallc.fmv.slot

//go:noinline
func llvmDynamicConcatPermuteInt8x16(x, y archsimd.Int8x16, indices archsimd.Uint8x16) archsimd.Int8x16 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Int8x16{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteUint8x16(x, y archsimd.Uint8x16, indices archsimd.Uint8x16) archsimd.Uint8x16 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Uint8x16{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteInt8x32(x, y archsimd.Int8x32, indices archsimd.Uint8x32) archsimd.Int8x32 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Int8x32{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteUint8x32(x, y archsimd.Uint8x32, indices archsimd.Uint8x32) archsimd.Uint8x32 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Uint8x32{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteInt8x64(x, y archsimd.Int8x64, indices archsimd.Uint8x64) archsimd.Int8x64 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Int8x64{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteUint8x64(x, y archsimd.Uint8x64, indices archsimd.Uint8x64) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Uint8x64{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteInt16x8(x, y archsimd.Int16x8, indices archsimd.Uint16x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x8{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteUint16x8(x, y archsimd.Uint16x8, indices archsimd.Uint16x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x8{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteInt16x16(x, y archsimd.Int16x16, indices archsimd.Uint16x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x16{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteUint16x16(x, y archsimd.Uint16x16, indices archsimd.Uint16x16) archsimd.Uint16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x16{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteInt16x32(x, y archsimd.Int16x32, indices archsimd.Uint16x32) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteUint16x32(x, y archsimd.Uint16x32, indices archsimd.Uint16x32) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteFloat32x4(x, y archsimd.Float32x4, indices archsimd.Uint32x4) archsimd.Float32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x4{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteInt32x4(x, y archsimd.Int32x4, indices archsimd.Uint32x4) archsimd.Int32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x4{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteUint32x4(x, y archsimd.Uint32x4, indices archsimd.Uint32x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x4{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteFloat32x8(x, y archsimd.Float32x8, indices archsimd.Uint32x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x8{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteInt32x8(x, y archsimd.Int32x8, indices archsimd.Uint32x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x8{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteUint32x8(x, y archsimd.Uint32x8, indices archsimd.Uint32x8) archsimd.Uint32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x8{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteFloat32x16(x, y archsimd.Float32x16, indices archsimd.Uint32x16) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteInt32x16(x, y archsimd.Int32x16, indices archsimd.Uint32x16) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteUint32x16(x, y archsimd.Uint32x16, indices archsimd.Uint32x16) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteFloat64x2(x, y archsimd.Float64x2, indices archsimd.Uint64x2) archsimd.Float64x2 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x2{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteInt64x2(x, y archsimd.Int64x2, indices archsimd.Uint64x2) archsimd.Int64x2 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x2{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteUint64x2(x, y archsimd.Uint64x2, indices archsimd.Uint64x2) archsimd.Uint64x2 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x2{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteFloat64x4(x, y archsimd.Float64x4, indices archsimd.Uint64x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x4{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteInt64x4(x, y archsimd.Int64x4, indices archsimd.Uint64x4) archsimd.Int64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x4{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteUint64x4(x, y archsimd.Uint64x4, indices archsimd.Uint64x4) archsimd.Uint64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x4{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteFloat64x8(x, y archsimd.Float64x8, indices archsimd.Uint64x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteInt64x8(x, y archsimd.Int64x8, indices archsimd.Uint64x8) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicConcatPermuteUint64x8(x, y archsimd.Uint64x8, indices archsimd.Uint64x8) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return x.ConcatPermute(y, indices)
}

//go:noinline
func llvmDynamicPermuteInt8x16(x, y archsimd.Int8x16, indices archsimd.Uint8x16) archsimd.Int8x16 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Int8x16{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteUint8x16(x, y archsimd.Uint8x16, indices archsimd.Uint8x16) archsimd.Uint8x16 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Uint8x16{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteInt8x32(x, y archsimd.Int8x32, indices archsimd.Uint8x32) archsimd.Int8x32 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Int8x32{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteUint8x32(x, y archsimd.Uint8x32, indices archsimd.Uint8x32) archsimd.Uint8x32 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Uint8x32{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteInt8x64(x, y archsimd.Int8x64, indices archsimd.Uint8x64) archsimd.Int8x64 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Int8x64{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteUint8x64(x, y archsimd.Uint8x64, indices archsimd.Uint8x64) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512VBMI() {
		return archsimd.Uint8x64{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteInt16x8(x, y archsimd.Int16x8, indices archsimd.Uint16x8) archsimd.Int16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x8{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteUint16x8(x, y archsimd.Uint16x8, indices archsimd.Uint16x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x8{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteInt16x16(x, y archsimd.Int16x16, indices archsimd.Uint16x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x16{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteUint16x16(x, y archsimd.Uint16x16, indices archsimd.Uint16x16) archsimd.Uint16x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x16{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteInt16x32(x, y archsimd.Int16x32, indices archsimd.Uint16x32) archsimd.Int16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int16x32{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteUint16x32(x, y archsimd.Uint16x32, indices archsimd.Uint16x32) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteFloat32x8(x, y archsimd.Float32x8, indices archsimd.Uint32x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Float32x8{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteInt32x8(x, y archsimd.Int32x8, indices archsimd.Uint32x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int32x8{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteUint32x8(x, y archsimd.Uint32x8, indices archsimd.Uint32x8) archsimd.Uint32x8 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint32x8{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteFloat32x16(x, y archsimd.Float32x16, indices archsimd.Uint32x16) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteInt32x16(x, y archsimd.Int32x16, indices archsimd.Uint32x16) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteUint32x16(x, y archsimd.Uint32x16, indices archsimd.Uint32x16) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteFloat64x4(x, y archsimd.Float64x4, indices archsimd.Uint64x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x4{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteInt64x4(x, y archsimd.Int64x4, indices archsimd.Uint64x4) archsimd.Int64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x4{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteUint64x4(x, y archsimd.Uint64x4, indices archsimd.Uint64x4) archsimd.Uint64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x4{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteFloat64x8(x, y archsimd.Float64x8, indices archsimd.Uint64x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteInt64x8(x, y archsimd.Int64x8, indices archsimd.Uint64x8) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteUint64x8(x, y archsimd.Uint64x8, indices archsimd.Uint64x8) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return x.Permute(indices)
}

//go:noinline
func llvmDynamicPermuteOrZeroInt8x16(x, y archsimd.Int8x16, indices archsimd.Int8x16) archsimd.Int8x16 {
	if !archsimd.X86.AVX() {
		return archsimd.Int8x16{}
	}
	return x.PermuteOrZero(indices)
}

//go:noinline
func llvmDynamicPermuteOrZeroUint8x16(x, y archsimd.Uint8x16, indices archsimd.Int8x16) archsimd.Uint8x16 {
	if !archsimd.X86.AVX() {
		return archsimd.Uint8x16{}
	}
	return x.PermuteOrZero(indices)
}

//go:noinline
func llvmDynamicPermuteOrZeroGroupedInt8x32(x, y archsimd.Int8x32, indices archsimd.Int8x32) archsimd.Int8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int8x32{}
	}
	return x.PermuteOrZeroGrouped(indices)
}

//go:noinline
func llvmDynamicPermuteOrZeroGroupedInt8x64(x, y archsimd.Int8x64, indices archsimd.Int8x64) archsimd.Int8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x64{}
	}
	return x.PermuteOrZeroGrouped(indices)
}

//go:noinline
func llvmDynamicPermuteOrZeroGroupedUint8x32(x, y archsimd.Uint8x32, indices archsimd.Int8x32) archsimd.Uint8x32 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint8x32{}
	}
	return x.PermuteOrZeroGrouped(indices)
}

//go:noinline
func llvmDynamicPermuteOrZeroGroupedUint8x64(x, y archsimd.Uint8x64, indices archsimd.Int8x64) archsimd.Uint8x64 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x64{}
	}
	return x.PermuteOrZeroGrouped(indices)
}
