// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "math"

// LLVM-LABEL: define goabiinternal double @codegen.llvmAbs64(double %x)
// LLVM-AMD64: and i64 {{%.*}}, 9223372036854775807
// LLVM-ARM64: call double @llvm.fabs.f64(double %x)
// LLVM-OPT-LABEL: define goabiinternal double @codegen.llvmAbs64(double %x)
// LLVM-OPT: call double @llvm.fabs.f64(double %x)
func llvmAbs64(x float64) float64 {
	return math.Abs(x)
}
