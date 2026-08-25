// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "math"

// The AMD64 SSA backend expands math.Abs into a sign-bit clear before LLVM,
// while ARM64 leaves an Abs operation for LLVM to lower.
// LLVM-LABEL: define goabiinternal double @codegen.llvmAbs64(double %x)
// LLVM-AMD64: and i64 {{%.*}}, 9223372036854775807
// LLVM-ARM64: call double @llvm.fabs.f64(double %x)
// Both forms canonicalize to llvm.fabs after LLVM optimization.
// LLVM-OPT-LABEL: define goabiinternal double @codegen.llvmAbs64(double %x)
// LLVM-OPT: call double @llvm.fabs.f64(double %x)
func llvmAbs64(x float64) float64 {
	return math.Abs(x)
}
