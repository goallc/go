// asmcheck -gcflags=-d=ssa/check/on

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "math"

// LLVM-DAG: define goabiinternal double @codegen.llvmTrunc64(double %x)
// LLVM-ARM64-DAG: call double @llvm.trunc.f64(double %x)
// LLVM-AMD64-DAG: call double @llvm.x86.go.sse41.round.f64(double %x, i32 3)
// LLVM-OPT-DAG: define goabiinternal double @codegen.llvmTrunc64(double %x)
// LLVM-OPT-ARM64-DAG: call double @llvm.trunc.f64(double %x)
// LLVM-OPT-AMD64-DAG: call double @llvm.x86.go.sse41.round.f64(double %x, i32 3)
// LLVM-AMD64-DAG: "target-cpu"="x86-64"
func llvmTrunc64(x float64) float64 {
	return math.Trunc(x)
}

// LLVM-DAG: define goabiinternal double @codegen.llvmCeil64(double %x)
// LLVM-ARM64-DAG: call double @llvm.ceil.f64(double %x)
// LLVM-AMD64-DAG: call double @llvm.x86.go.sse41.round.f64(double %x, i32 2)
func llvmCeil64(x float64) float64 {
	return math.Ceil(x)
}

// LLVM-DAG: define goabiinternal double @codegen.llvmFloor64(double %x)
// LLVM-ARM64-DAG: call double @llvm.floor.f64(double %x)
// LLVM-AMD64-DAG: call double @llvm.x86.go.sse41.round.f64(double %x, i32 1)
func llvmFloor64(x float64) float64 {
	return math.Floor(x)
}

// LLVM-DAG: define goabiinternal double @codegen.llvmRound64(double %x)
// LLVM-DAG: call double @llvm.round.f64(double %x)
func llvmRound64(x float64) float64 {
	return math.Round(x)
}

// LLVM-DAG: define goabiinternal double @codegen.llvmRoundToEven64(double %x)
// LLVM-ARM64-DAG: call double @llvm.roundeven.f64(double %x)
// LLVM-AMD64-DAG: call double @llvm.x86.go.sse41.round.f64(double %x, i32 0)
func llvmRoundToEven64(x float64) float64 {
	return math.RoundToEven(x)
}
