// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "math"

// LLVM-ARM64-DAG: call double @llvm.fma.f64(double %x, double %y, double %z)
// LLVM-AMD64-DAG: call double @llvm.fma.f64(double %x, double %y, double %z){{.*}}!goallc.cpu.requires
func llvmFMA64(x, y, z float64) float64 {
	return math.FMA(x, y, z)
}

// LLVM-DAG: call double @llvm.minimum.f64(double %x, double %y)
func llvmMin64(x, y float64) float64 {
	return min(x, y)
}

// LLVM-DAG: call double @llvm.maximum.f64(double %x, double %y)
func llvmMax64(x, y float64) float64 {
	return max(x, y)
}

// LLVM-DAG: call float @llvm.minimum.f32(float %x, float %y)
func llvmMin32(x, y float32) float32 {
	return min(x, y)
}

// LLVM-DAG: call float @llvm.maximum.f32(float %x, float %y)
func llvmMax32(x, y float32) float32 {
	return max(x, y)
}
