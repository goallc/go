// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && amd64

package codegen

import "simd/archsimd"

// LLVM-ASM-AMD64-DAG: VCVTPD2PSX X1, X0
// LLVM-ASM-AMD64-DAG: VCVTPD2PSY Y1, X0
// LLVM-ASM-AMD64-DAG: VCVTPD2PS Z1, Y0
// LLVM-ASM-AMD64-DAG: VCVTDQ2PS X1, X0
// LLVM-ASM-AMD64-DAG: VCVTDQ2PS Y1, Y0
// LLVM-ASM-AMD64-DAG: VCVTDQ2PS Z1, Z0
// LLVM-ASM-AMD64-DAG: VCVTQQ2PS Z1, Y0
// LLVM-ASM-AMD64-DAG: VCVTUDQ2PS Z1, Z0
// LLVM-ASM-AMD64-DAG: VCVTUQQ2PS Z1, Y0
// LLVM-ASM-AMD64-DAG: VCVTPS2PD X1, Y0
// LLVM-ASM-AMD64-DAG: VCVTPS2PD Y1, Z0
// LLVM-ASM-AMD64-DAG: VCVTDQ2PD X1, Y0
// LLVM-ASM-AMD64-DAG: VCVTUDQ2PD Y1, Z0
// LLVM-ASM-AMD64-DAG: VCVTQQ2PD Z1, Z0
// LLVM-ASM-AMD64-DAG: VCVTUQQ2PD Z1, Z0
// LLVM-ASM-AMD64-DAG: VCVTTPS2DQ X0, X0
// LLVM-ASM-AMD64-DAG: VCVTTPS2DQ Y0, Y0
// LLVM-ASM-AMD64-DAG: VCVTTPS2DQ Z0, Z0
// LLVM-ASM-AMD64-DAG: VCVTTPD2DQX X0, X0
// LLVM-ASM-AMD64-DAG: VCVTTPD2DQY Y0, X0
// LLVM-ASM-AMD64-DAG: VCVTTPD2DQ Z0, Y0
// LLVM-ASM-AMD64-DAG: VCVTTPS2QQ X0, Y0
// LLVM-ASM-AMD64-DAG: VCVTTPS2QQ Y0, Z0
// LLVM-ASM-AMD64-DAG: VCVTTPD2QQ X0, X0
// LLVM-ASM-AMD64-DAG: VCVTTPD2QQ Z0, Z0
// LLVM-ASM-AMD64-DAG: VCVTTPS2UDQ X0, X0
// LLVM-ASM-AMD64-DAG: VCVTTPS2UDQ Z0, Z0
// LLVM-ASM-AMD64-DAG: VCVTTPD2UDQ Z0, Y0
// LLVM-ASM-AMD64-DAG: VCVTTPS2UQQ Y0, Z0
// LLVM-ASM-AMD64-DAG: VCVTTPD2UQQ Z0, Z0

// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmConvertToUint32Float32x4<goallc.fmv.baseline>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmConvertToUint32Float32x4<goallc.fmv.avx512>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmConvertToUint32Float32x4<goallc.fmv.resolve>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmConvertToInt64Float32x4<goallc.fmv.baseline>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmConvertToInt64Float32x4<goallc.fmv.avx512>"
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.llvmConvertToInt64Float32x4<goallc.fmv.resolve>"
// LLVM-NM-AMD64: codegen.llvmConvertToInt64Float32x4.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-3: codegen.llvmConvertToInt64Float32x4<1>
// LLVM-NM-AMD64: codegen.llvmConvertToUint32Float32x4.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-3: codegen.llvmConvertToUint32Float32x4<1>

// The pre-optimization module emits these functions in reverse declaration order.
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToUint64Float64x8(
// LLVM-AMD64: call <8 x i64> @llvm.x86.avx512.mask.cvttpd2uqq.512(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToUint64Float64x4(
// LLVM-AMD64: call <4 x i64> @llvm.x86.avx512.mask.cvttpd2uqq.256(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToUint64Float64x2(
// LLVM-AMD64: call <2 x i64> @llvm.x86.avx512.mask.cvttpd2uqq.128(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToUint64Float32x8(
// LLVM-AMD64: call <8 x i64> @llvm.x86.avx512.mask.cvttps2uqq.512(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToUint64Float32x4(
// LLVM-AMD64: call <4 x i64> @llvm.x86.avx512.mask.cvttps2uqq.256(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToUint32Float64x8(
// LLVM-AMD64: call <8 x i32> @llvm.x86.avx512.mask.cvttpd2udq.512(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToUint32Float64x4(
// LLVM-AMD64: call <4 x i32> @llvm.x86.avx512.mask.cvttpd2udq.256(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToUint32Float64x2(
// LLVM-AMD64: call <4 x i32> @llvm.x86.avx512.mask.cvttpd2udq.128(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToUint32Float32x16(
// LLVM-AMD64: call <16 x i32> @llvm.x86.avx512.mask.cvttps2udq.512(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToUint32Float32x8(
// LLVM-AMD64: call <8 x i32> @llvm.x86.avx512.mask.cvttps2udq.256(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToUint32Float32x4(
// LLVM-AMD64: call <4 x i32> @llvm.x86.avx512.mask.cvttps2udq.128(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToInt64Float64x8(
// LLVM-AMD64: call <8 x i64> @llvm.x86.avx512.mask.cvttpd2qq.512(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToInt64Float64x4(
// LLVM-AMD64: call <4 x i64> @llvm.x86.avx512.mask.cvttpd2qq.256(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToInt64Float64x2(
// LLVM-AMD64: call <2 x i64> @llvm.x86.avx512.mask.cvttpd2qq.128(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToInt64Float32x8(
// LLVM-AMD64: call <8 x i64> @llvm.x86.avx512.mask.cvttps2qq.512(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToInt64Float32x4(
// LLVM-AMD64: call <4 x i64> @llvm.x86.avx512.mask.cvttps2qq.256(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToInt32Float64x8(
// LLVM-AMD64: call <8 x i32> @llvm.x86.avx512.mask.cvttpd2dq.512(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToInt32Float64x4(
// LLVM-AMD64: call <4 x i32> @llvm.x86.avx.cvtt.pd2dq.256(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToInt32Float64x2(
// LLVM-AMD64: call <4 x i32> @llvm.x86.sse2.cvttpd2dq(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToInt32Float32x16(
// LLVM-AMD64: call <16 x i32> @llvm.x86.avx512.mask.cvttps2dq.512(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToInt32Float32x8(
// LLVM-AMD64: call <8 x i32> @llvm.x86.avx.cvtt.ps2dq.256(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToInt32Float32x4(
// LLVM-AMD64: call <4 x i32> @llvm.x86.sse2.cvttps2dq(
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat64Uint64x8(
// LLVM-AMD64: uitofp <8 x i64> {{.*}} to <8 x double>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat64Uint64x4(
// LLVM-AMD64: uitofp <4 x i64> {{.*}} to <4 x double>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat64Uint64x2(
// LLVM-AMD64: uitofp <2 x i64> {{.*}} to <2 x double>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat64Uint32x8(
// LLVM-AMD64: uitofp <8 x i32> {{.*}} to <8 x double>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat64Uint32x4(
// LLVM-AMD64: uitofp <4 x i32> {{.*}} to <4 x double>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat64Int64x8(
// LLVM-AMD64: sitofp <8 x i64> {{.*}} to <8 x double>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat64Int64x4(
// LLVM-AMD64: sitofp <4 x i64> {{.*}} to <4 x double>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat64Int64x2(
// LLVM-AMD64: sitofp <2 x i64> {{.*}} to <2 x double>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat64Int32x8(
// LLVM-AMD64: sitofp <8 x i32> {{.*}} to <8 x double>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat64Int32x4(
// LLVM-AMD64: sitofp <4 x i32> {{.*}} to <4 x double>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat64Float32x8(
// LLVM-AMD64: fpext <8 x float> {{.*}} to <8 x double>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat64Float32x4(
// LLVM-AMD64: fpext <4 x float> {{.*}} to <4 x double>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Uint64x8(
// LLVM-AMD64: uitofp <8 x i64> {{.*}} to <8 x float>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Uint64x4(
// LLVM-AMD64: uitofp <4 x i64> {{.*}} to <4 x float>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Uint64x2(
// LLVM-AMD64: uitofp <2 x i64> {{.*}} to <2 x float>
// LLVM-AMD64: shufflevector <2 x float> {{.*}}, <2 x float> zeroinitializer
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Uint32x16(
// LLVM-AMD64: uitofp <16 x i32> {{.*}} to <16 x float>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Uint32x8(
// LLVM-AMD64: uitofp <8 x i32> {{.*}} to <8 x float>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Uint32x4(
// LLVM-AMD64: uitofp <4 x i32> {{.*}} to <4 x float>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Int64x8(
// LLVM-AMD64: sitofp <8 x i64> {{.*}} to <8 x float>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Int64x4(
// LLVM-AMD64: sitofp <4 x i64> {{.*}} to <4 x float>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Int64x2(
// LLVM-AMD64: sitofp <2 x i64> {{.*}} to <2 x float>
// LLVM-AMD64: shufflevector <2 x float> {{.*}}, <2 x float> zeroinitializer
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Int32x16(
// LLVM-AMD64: sitofp <16 x i32> {{.*}} to <16 x float>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Int32x8(
// LLVM-AMD64: sitofp <8 x i32> {{.*}} to <8 x float>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Int32x4(
// LLVM-AMD64: sitofp <4 x i32> {{.*}} to <4 x float>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Float64x8(
// LLVM-AMD64: fptrunc <8 x double> {{.*}} to <8 x float>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Float64x4(
// LLVM-AMD64: fptrunc <4 x double> {{.*}} to <4 x float>
// LLVM-AMD64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Float64x2(
// LLVM-AMD64: fptrunc <2 x double> {{.*}} to <2 x float>
// LLVM-AMD64: shufflevector <2 x float> {{.*}}, <2 x float> zeroinitializer

//go:noinline
func llvmConvertToFloat32Float64x2(x archsimd.Float64x2) archsimd.Float32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x4{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat32Float64x4(x archsimd.Float64x4) archsimd.Float32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x4{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat32Float64x8(x archsimd.Float64x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x8{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat32Int32x4(x archsimd.Int32x4) archsimd.Float32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x4{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat32Int32x8(x archsimd.Int32x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Float32x8{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat32Int32x16(x archsimd.Int32x16) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat32Int64x2(x archsimd.Int64x2) archsimd.Float32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x4{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat32Int64x4(x archsimd.Int64x4) archsimd.Float32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x4{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat32Int64x8(x archsimd.Int64x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x8{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat32Uint32x4(x archsimd.Uint32x4) archsimd.Float32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x4{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat32Uint32x8(x archsimd.Uint32x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x8{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat32Uint32x16(x archsimd.Uint32x16) archsimd.Float32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x16{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat32Uint64x2(x archsimd.Uint64x2) archsimd.Float32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x4{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat32Uint64x4(x archsimd.Uint64x4) archsimd.Float32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x4{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat32Uint64x8(x archsimd.Uint64x8) archsimd.Float32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float32x8{}
	}
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat64Float32x4(x archsimd.Float32x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x4{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func llvmConvertToFloat64Float32x8(x archsimd.Float32x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func llvmConvertToFloat64Int32x4(x archsimd.Int32x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Float64x4{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func llvmConvertToFloat64Int32x8(x archsimd.Int32x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func llvmConvertToFloat64Int64x2(x archsimd.Int64x2) archsimd.Float64x2 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x2{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func llvmConvertToFloat64Int64x4(x archsimd.Int64x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x4{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func llvmConvertToFloat64Int64x8(x archsimd.Int64x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func llvmConvertToFloat64Uint32x4(x archsimd.Uint32x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x4{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func llvmConvertToFloat64Uint32x8(x archsimd.Uint32x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func llvmConvertToFloat64Uint64x2(x archsimd.Uint64x2) archsimd.Float64x2 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x2{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func llvmConvertToFloat64Uint64x4(x archsimd.Uint64x4) archsimd.Float64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x4{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func llvmConvertToFloat64Uint64x8(x archsimd.Uint64x8) archsimd.Float64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Float64x8{}
	}
	return x.ConvertToFloat64()
}

//go:noinline
func llvmConvertToInt32Float32x4(x archsimd.Float32x4) archsimd.Int32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x4{}
	}
	return x.ConvertToInt32()
}

//go:noinline
func llvmConvertToInt32Float32x8(x archsimd.Float32x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x8{}
	}
	return x.ConvertToInt32()
}

//go:noinline
func llvmConvertToInt32Float32x16(x archsimd.Float32x16) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return x.ConvertToInt32()
}

//go:noinline
func llvmConvertToInt32Float64x2(x archsimd.Float64x2) archsimd.Int32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x4{}
	}
	return x.ConvertToInt32()
}

//go:noinline
func llvmConvertToInt32Float64x4(x archsimd.Float64x4) archsimd.Int32x4 {
	if !archsimd.X86.AVX() {
		return archsimd.Int32x4{}
	}
	return x.ConvertToInt32()
}

//go:noinline
func llvmConvertToInt32Float64x8(x archsimd.Float64x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x8{}
	}
	return x.ConvertToInt32()
}

//go:noinline
func llvmConvertToInt64Float32x4(x archsimd.Float32x4) archsimd.Int64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x4{}
	}
	return x.ConvertToInt64()
}

//go:noinline
func llvmConvertToInt64Float32x8(x archsimd.Float32x8) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return x.ConvertToInt64()
}

//go:noinline
func llvmConvertToInt64Float64x2(x archsimd.Float64x2) archsimd.Int64x2 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x2{}
	}
	return x.ConvertToInt64()
}

//go:noinline
func llvmConvertToInt64Float64x4(x archsimd.Float64x4) archsimd.Int64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x4{}
	}
	return x.ConvertToInt64()
}

//go:noinline
func llvmConvertToInt64Float64x8(x archsimd.Float64x8) archsimd.Int64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int64x8{}
	}
	return x.ConvertToInt64()
}

//go:noinline
func llvmConvertToUint32Float32x4(x archsimd.Float32x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x4{}
	}
	return x.ConvertToUint32()
}

//go:noinline
func llvmConvertToUint32Float32x8(x archsimd.Float32x8) archsimd.Uint32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x8{}
	}
	return x.ConvertToUint32()
}

//go:noinline
func llvmConvertToUint32Float32x16(x archsimd.Float32x16) archsimd.Uint32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x16{}
	}
	return x.ConvertToUint32()
}

//go:noinline
func llvmConvertToUint32Float64x2(x archsimd.Float64x2) archsimd.Uint32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x4{}
	}
	return x.ConvertToUint32()
}

//go:noinline
func llvmConvertToUint32Float64x4(x archsimd.Float64x4) archsimd.Uint32x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x4{}
	}
	return x.ConvertToUint32()
}

//go:noinline
func llvmConvertToUint32Float64x8(x archsimd.Float64x8) archsimd.Uint32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint32x8{}
	}
	return x.ConvertToUint32()
}

//go:noinline
func llvmConvertToUint64Float32x4(x archsimd.Float32x4) archsimd.Uint64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x4{}
	}
	return x.ConvertToUint64()
}

//go:noinline
func llvmConvertToUint64Float32x8(x archsimd.Float32x8) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return x.ConvertToUint64()
}

//go:noinline
func llvmConvertToUint64Float64x2(x archsimd.Float64x2) archsimd.Uint64x2 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x2{}
	}
	return x.ConvertToUint64()
}

//go:noinline
func llvmConvertToUint64Float64x4(x archsimd.Float64x4) archsimd.Uint64x4 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x4{}
	}
	return x.ConvertToUint64()
}

//go:noinline
func llvmConvertToUint64Float64x8(x archsimd.Float64x8) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return x.ConvertToUint64()
}
