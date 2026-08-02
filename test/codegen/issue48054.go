// asmcheck

// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-DAG: define goabiinternal i8 @codegen.a3({ ptr, i64, i64 } %n)
// LLVM-DAG: extractvalue { ptr, i64, i64 } %n, 1
// LLVM-DAG: icmp eq i64 0, {{%.*}}
// LLVM-DAG: define goabiinternal i8 @codegen.a({ ptr, i64 } %n)
// LLVM-DAG: extractvalue { ptr, i64 } %n, 1
// LLVM-DAG: icmp ne i64 0, {{%.*}}

func a(n string) bool {
	// arm64:"CBZ"
	if len(n) > 0 {
		return true
	}
	return false
}

func a2(n []int) bool {
	// arm64:"CBZ"
	if len(n) > 0 {
		return true
	}
	return false
}

func a3(n []int) bool {
	// amd64:"TESTQ"
	if len(n) < 1 {
		return true
	}
	return false
}
