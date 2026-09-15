// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-LABEL: define {{.*}} @codegen.llvmBranchOnlyFloatBool(
// LLVM: fcmp olt
// LLVM-NOT: zext i1
// LLVM: br i1
func llvmBranchOnlyFloatBool(x, y float64) int {
	if x < y {
		return 1
	}
	return 2
}
