// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "slices"

// Imported generic bodies may create the same closure in multiple packages.
// Its GoObj identity must use the hashed definition class and its canonical
// name, not the temporary inline-call-stack hash that distinguishes compiler
// instances before content hashing.
func llvmContentAddressableClosure(values []int) int {
	for value := range slices.Values(values) {
		return value
	}
	return 0
}

// LLVM-NATIVE-OBJSUMMARY-DAG: NATIVE symbol name="slices.Values[go.shape.[]int,go.shape.int].func1" kind=STEXT flags={{.*}} class=hashed hash={{[0-9a-f]+}}
// LLVM-NATIVE-OBJSUMMARY-DAG: LLVM symbol name="slices.Values[go.shape.[]int,go.shape.int].func1" kind=STEXT flags={{.*}} class=hashed hash={{[0-9a-f]+}}
// LLVM-NATIVE-OBJSUMMARY-NOT: slices.Values[go.shape.[]int,go.shape.int].func1#
