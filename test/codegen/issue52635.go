// asmcheck

// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test that optimized range memclr works with pointers to arrays.
// The clears get inlined, see https://github.com/golang/go/issues/56997

package codegen

// LLVM-DAG: define goabiinternal void @"codegen.(*T).f"(ptr %t)
// LLVM-DAG: call goabiinternal void @"runtime.memclrNoHeapPointers<builtin.{{[0-9]+}}>"(ptr {{%.*}}, i64 80)
// LLVM-DAG: [[LENADDR:%.*]] = getelementptr i8, ptr %t, i64 96
// LLVM-DAG: [[LEN:%.*]] = load i64, ptr [[LENADDR]]
// LLVM-DAG: [[BYTES:%.*]] = shl i64 [[LEN]], 3
// LLVM-DAG: call goabiinternal void @"runtime.memclrNoHeapPointers<builtin.{{[0-9]+}}>"(ptr {{%.*}}, i64 [[BYTES]])

type T struct {
	a *[10]int
	b [10]int
	s []int
}

func (t *T) f() {
	for i := range t.a {
		// amd64:-".*runtime.memclrNoHeapPointers"
		// amd64:`MOVUPS X15,`
		t.a[i] = 0
	}

	for i := range *t.a {
		// amd64:-".*runtime.memclrNoHeapPointers"
		// amd64:`MOVUPS X15,`
		t.a[i] = 0
	}

	for i := range t.a {
		// amd64:-".*runtime.memclrNoHeapPointers"
		// amd64:`MOVUPS X15,`
		(*t.a)[i] = 0
	}

	for i := range *t.a {
		// amd64:-".*runtime.memclrNoHeapPointers"
		// amd64:`MOVUPS X15,`
		(*t.a)[i] = 0
	}

	// amd64:-".*runtime.memclrNoHeapPointers"
	// amd64:`MOVUPS X15,`
	for i := range t.b {
		t.b[i] = 0
	}

	// amd64:".*runtime.memclrNoHeapPointers"
	for i := range t.s {
		t.s[i] = 0
	}
}
