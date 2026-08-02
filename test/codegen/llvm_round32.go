// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// An explicit float32 conversion is a Go rounding boundary. LLVM float is
// already binary32, so Round32F is an identity in IR, but it must not enable
// contraction of the multiply and add on either supported target.
//
// LLVM-LABEL: define goabiinternal float @codegen.llvmRound32NoContract(float %x, float %y, float %z)
// LLVM: [[PRODUCT:%.*]] = fmul float %x, %y
// LLVM-NEXT: [[SUM:%.*]] = fadd float [[PRODUCT]], %z
// LLVM-NEXT: ret float [[SUM]]
// LLVM-NOT: llvm.fma
// LLVM-OPT-LABEL: define goabiinternal float @codegen.llvmRound32NoContract(float %x, float %y, float %z)
// LLVM-OPT: [[OPT_PRODUCT:%.*]] = fmul float %x, %y
// LLVM-OPT-NEXT: [[OPT_SUM:%.*]] = fadd float [[OPT_PRODUCT]], %z
// LLVM-OPT-NEXT: ret float [[OPT_SUM]]
// LLVM-OPT-NOT: llvm.fma
func llvmRound32NoContract(x, y, z float32) float32 {
	return float32(x*y) + z
}
