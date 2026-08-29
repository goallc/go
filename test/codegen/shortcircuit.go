// asmcheck

// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-LABEL: define goabiinternal i64 @codegen.efaceExtract(
// LLVM: extractvalue { i64, ptr } %e, 0
// LLVM: icmp eq i64 %{{.*}}, ptrtoint (ptr @"type:int" to i64)
// LLVM: extractvalue { i64, ptr } %e, 1

func efaceExtract(e interface{}) int {
	// This should be compiled with only
	// a single conditional jump.
	// amd64:-"JMP"
	if x, ok := e.(int); ok {
		return x
	}
	return 0
}
