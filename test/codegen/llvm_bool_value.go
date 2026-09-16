// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-LABEL: define {{.*}} @codegen.llvmBoolValue(
// LLVM-DAG: load i8
// LLVM-DAG: icmp ne i8
// LLVM-DAG: phi i1
// LLVM-DAG: icmp ne i1
// LLVM-DAG: zext i1 {{.*}} to i8
// LLVM-DAG: store i8
func llvmBoolValue(p *bool, n int) bool {
	b := *p
	for i := 0; i < n; i++ {
		b = b != (i&1 == 0)
	}
	*p = b
	return b
}

// LLVM-LABEL: define goabiinternal i1 @codegen.llvmBoolIdentity(i1 %b)
// LLVM-NOT: icmp
// LLVM-NOT: zext
// LLVM: ret i1 %b
func llvmBoolIdentity(b bool) bool { return b }
