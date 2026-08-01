// asmcheck

// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-LABEL: define goabiinternal ptr @codegen.f(
// LLVM-DAG: icmp ult i64 %x, 4
// LLVM-DAG: getelementptr [2 x i64], ptr %p, i64 %x
// LLVM-DAG: getelementptr i64, ptr

// Make sure we use ADDQ instead of LEAQ when we can.

func f(p *[4][2]int, x int) *int {
	// amd64:"ADDQ" -"LEAQ"
	return &p[x][0]
}
