// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && arm64

package codegen

import "simd/archsimd"

// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmConvertLo2ToFloat64Float32x4(SB)
// LLVM-ASM-ARM64: VFCVTL V0.S2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmConvertToFloat32Float64x2(SB)
// LLVM-ASM-ARM64: VFCVTN V0.D2, V0.S2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmConvertToFloat32Int32x4(SB)
// LLVM-ASM-ARM64: SCVTF V0.S4, V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmConvertToFloat32Uint32x4(SB)
// LLVM-ASM-ARM64: UCVTF V0.S4, V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmConvertToFloat64Int64x2(SB)
// LLVM-ASM-ARM64: SCVTF V0.D2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmConvertToFloat64Uint64x2(SB)
// LLVM-ASM-ARM64: UCVTF V0.D2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmConvertToInt32Float32x4(SB)
// LLVM-ASM-ARM64: FCVTZS V0.S4, V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmConvertToInt64Float64x2(SB)
// LLVM-ASM-ARM64: FCVTZS V0.D2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmConvertToUint32Float32x4(SB)
// LLVM-ASM-ARM64: FCVTZU V0.S4, V0.S4
// LLVM-ASM-ARM64-NEXT: {{.*}}RET
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmConvertToUint64Float64x2(SB)
// LLVM-ASM-ARM64: FCVTZU V0.D2, V0.D2
// LLVM-ASM-ARM64-NEXT: {{.*}}RET

// The pre-optimization module emits these functions in reverse declaration order.
// LLVM-ARM64-LABEL: define {{.*}} @codegen.llvmConvertToUint64Float64x2(
// LLVM-ARM64: call <2 x i64> @llvm.fptoui.sat.v2i64.v2f64(
// LLVM-ARM64-LABEL: define {{.*}} @codegen.llvmConvertToUint32Float32x4(
// LLVM-ARM64: call <4 x i32> @llvm.fptoui.sat.v4i32.v4f32(
// LLVM-ARM64-LABEL: define {{.*}} @codegen.llvmConvertToInt64Float64x2(
// LLVM-ARM64: call <2 x i64> @llvm.fptosi.sat.v2i64.v2f64(
// LLVM-ARM64-LABEL: define {{.*}} @codegen.llvmConvertToInt32Float32x4(
// LLVM-ARM64: call <4 x i32> @llvm.fptosi.sat.v4i32.v4f32(
// LLVM-ARM64-LABEL: define {{.*}} @codegen.llvmConvertToFloat64Uint64x2(
// LLVM-ARM64: uitofp <2 x i64> {{.*}} to <2 x double>
// LLVM-ARM64-LABEL: define {{.*}} @codegen.llvmConvertToFloat64Int64x2(
// LLVM-ARM64: sitofp <2 x i64> {{.*}} to <2 x double>
// LLVM-ARM64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Uint32x4(
// LLVM-ARM64: uitofp <4 x i32> {{.*}} to <4 x float>
// LLVM-ARM64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Int32x4(
// LLVM-ARM64: sitofp <4 x i32> {{.*}} to <4 x float>
// LLVM-ARM64-LABEL: define {{.*}} @codegen.llvmConvertToFloat32Float64x2(
// LLVM-ARM64: fptrunc <2 x double> {{.*}} to <2 x float>
// LLVM-ARM64: shufflevector <2 x float> {{.*}}, <2 x float> zeroinitializer
// LLVM-ARM64-LABEL: define {{.*}} @codegen.llvmConvertLo2ToFloat64Float32x4(
// LLVM-ARM64: fpext <2 x float> {{.*}} to <2 x double>

//go:noinline
func llvmConvertLo2ToFloat64Float32x4(x archsimd.Float32x4) archsimd.Float64x2 {
	return x.ConvertLo2ToFloat64()
}

//go:noinline
func llvmConvertToFloat32Float64x2(x archsimd.Float64x2) archsimd.Float32x4 {
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat32Int32x4(x archsimd.Int32x4) archsimd.Float32x4 {
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat32Uint32x4(x archsimd.Uint32x4) archsimd.Float32x4 {
	return x.ConvertToFloat32()
}

//go:noinline
func llvmConvertToFloat64Int64x2(x archsimd.Int64x2) archsimd.Float64x2 {
	return x.ConvertToFloat64()
}

//go:noinline
func llvmConvertToFloat64Uint64x2(x archsimd.Uint64x2) archsimd.Float64x2 {
	return x.ConvertToFloat64()
}

//go:noinline
func llvmConvertToInt32Float32x4(x archsimd.Float32x4) archsimd.Int32x4 {
	return x.ConvertToInt32()
}

//go:noinline
func llvmConvertToInt64Float64x2(x archsimd.Float64x2) archsimd.Int64x2 {
	return x.ConvertToInt64()
}

//go:noinline
func llvmConvertToUint32Float32x4(x archsimd.Float32x4) archsimd.Uint32x4 {
	return x.ConvertToUint32()
}

//go:noinline
func llvmConvertToUint64Float64x2(x archsimd.Float64x2) archsimd.Uint64x2 {
	return x.ConvertToUint64()
}
