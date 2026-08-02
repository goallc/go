// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// An explicit float64 conversion is a Go rounding boundary. LLVM double is
// already binary64, so Round64F is an identity in IR, but it must not enable
// contraction of the multiply and add on either supported target.
//
// LLVM-LABEL: define goabiinternal double @codegen.llvmRound64NoContract(double %x, double %y, double %z)
// LLVM: [[PRODUCT:%.*]] = fmul double %x, %y
// LLVM-NEXT: [[SUM:%.*]] = fadd double [[PRODUCT]], %z
// LLVM-NEXT: ret double [[SUM]]
// LLVM-NOT: llvm.fma
// LLVM-OPT-LABEL: define goabiinternal double @codegen.llvmRound64NoContract(double %x, double %y, double %z)
// LLVM-OPT: [[OPT_PRODUCT:%.*]] = fmul double %x, %y
// LLVM-OPT-NEXT: [[OPT_SUM:%.*]] = fadd double [[OPT_PRODUCT]], %z
// LLVM-OPT-NEXT: ret double [[OPT_SUM]]
// LLVM-OPT-NOT: llvm.fma
func llvmRound64NoContract(x, y, z float64) float64 {
	return float64(x*y) + z
}
