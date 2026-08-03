// asmcheck -gcflags=-d=ssa/check/on

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "math"

// LLVM-LABEL: define goabiinternal double @codegen.llvmTrunc64(double %x)
// LLVM: call double @llvm.trunc.f64(double %x)
// LLVM-OPT-LABEL: define goabiinternal double @codegen.llvmTrunc64(double %x)
// LLVM-OPT: call double @llvm.trunc.f64(double %x)
func llvmTrunc64(x float64) float64 {
	return math.Trunc(x)
}
