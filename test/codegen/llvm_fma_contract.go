// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM: target triple
// LLVM-OPT: target triple

// These are the floating-point expression shapes contracted by the native
// ARM64 SSA rules. The LLVM backend must express the same contraction without
// enabling reassociation or any other fast-math operation.

// LLVM-ARM64-LABEL: define goabiinternal double @codegen.llvmFMA64Add(
// LLVM-ARM64: fmul contract double
// LLVM-ARM64: fadd contract double
// LLVM-OPT-ARM64-LABEL: define goabiinternal double @codegen.llvmFMA64Add(
// LLVM-OPT-ARM64: fmul contract double
// LLVM-OPT-ARM64: fadd contract double
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmFMA64Add(SB)
// LLVM-ASM-ARM64-NEXT: {{.*}}F{{(N?MADD|N?MSUB)}}D
// LLVM-ASM-AMD64-LABEL: TEXT codegen.llvmFMA64Add(SB)
// LLVM-ASM-AMD64: MULSD
// LLVM-ASM-AMD64-NEXT: {{.*}}ADDSD
func llvmFMA64Add(a, b, c float64) float64 { return a*b + c }

// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmFMA64MulSub(SB)
// LLVM-ASM-ARM64-NEXT: {{.*}}F{{(N?MADD|N?MSUB)}}D
func llvmFMA64MulSub(a, b, c float64) float64 { return a*b - c }

// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmFMA64SubMul(SB)
// LLVM-ASM-ARM64-NEXT: {{.*}}F{{(N?MADD|N?MSUB)}}D
func llvmFMA64SubMul(a, b, c float64) float64 { return c - a*b }

// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmFMA64NegMulAdd(SB)
// LLVM-ASM-ARM64-NEXT: {{.*}}F{{(N?MADD|N?MSUB)}}D
func llvmFMA64NegMulAdd(a, b, c float64) float64 { return -(a * b) + c }

// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmFMA64NegMulSub(SB)
// LLVM-ASM-ARM64-NEXT: {{.*}}F{{(N?MADD|N?MSUB)}}D
func llvmFMA64NegMulSub(a, b, c float64) float64 { return -(a * b) - c }

// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmFMA32Add(SB)
// LLVM-ASM-ARM64-NEXT: {{.*}}F{{(N?MADD|N?MSUB)}}S
// LLVM-ASM-AMD64-LABEL: TEXT codegen.llvmFMA32Add(SB)
// LLVM-ASM-AMD64: MULSS
// LLVM-ASM-AMD64-NEXT: {{.*}}ADDSS
func llvmFMA32Add(a, b, c float32) float32 { return a*b + c }

// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmFMA32MulSub(SB)
// LLVM-ASM-ARM64-NEXT: {{.*}}F{{(N?MADD|N?MSUB)}}S
func llvmFMA32MulSub(a, b, c float32) float32 { return a*b - c }

// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmFMA32SubMul(SB)
// LLVM-ASM-ARM64-NEXT: {{.*}}F{{(N?MADD|N?MSUB)}}S
func llvmFMA32SubMul(a, b, c float32) float32 { return c - a*b }

// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmFMA32NegMulAdd(SB)
// LLVM-ASM-ARM64-NEXT: {{.*}}F{{(N?MADD|N?MSUB)}}S
func llvmFMA32NegMulAdd(a, b, c float32) float32 { return -(a * b) + c }

// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmFMA32NegMulSub(SB)
// LLVM-ASM-ARM64-NEXT: {{.*}}F{{(N?MADD|N?MSUB)}}S
func llvmFMA32NegMulSub(a, b, c float32) float32 { return -(a * b) - c }

// Explicit conversions are Go rounding boundaries even when their source and
// destination types are identical.
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmFMA64RoundBoundary(SB)
// LLVM-ASM-ARM64: FMULD
// LLVM-ASM-ARM64-NEXT: {{.*}}FADDD
func llvmFMA64RoundBoundary(a, b, c float64) float64 { return float64(a*b) + c }

// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmFMA32RoundBoundary(SB)
// LLVM-ASM-ARM64: FMULS
// LLVM-ASM-ARM64-NEXT: {{.*}}FADDS
func llvmFMA32RoundBoundary(a, b, c float32) float32 { return float32(a*b) + c }

// Contraction must not reassociate the outer addition.
// LLVM-ASM-ARM64-LABEL: TEXT codegen.llvmFMANoReassociate(SB)
// LLVM-ASM-ARM64: F{{(N?MADD|N?MSUB)}}D
// LLVM-ASM-ARM64-NEXT: {{.*}}FADDD
func llvmFMANoReassociate(a, b, c, d float64) float64 { return (a*b + c) + d }

// LLVM-ASM: RET
