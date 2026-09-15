// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-LABEL: define {{.*}} @codegen.llvmBranchOnlyBool(
// LLVM: icmp slt
// LLVM-NOT: zext i1
// LLVM: br i1
func llvmBranchOnlyBool(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func llvmBranchFloatBool(x, y float64) int {
	if x < y {
		return 1
	}
	return 2
}

func llvmMixedBool(x, y int, out *bool) int {
	b := x < y
	*out = b
	if b {
		return x
	}
	return y
}

func llvmReturnBool(x, y int) bool { return x < y }

func llvmPhiBool(x, y int, b bool) bool {
	if x < 0 {
		b = y < 0
	}
	return b
}
