// asmcheck

package codegen

// LLVM-DAG: @runtime.staticuint64s = external global i8
// LLVM-DAG: define goabiinternal { i64, ptr } @codegen.booliface()
// LLVM-DAG: define goabiinternal { i64, ptr } @codegen.smallint8iface()
// LLVM-DAG: define goabiinternal { i64, ptr } @codegen.smalluint8iface()
// LLVM-DAG: getelementptr i8, ptr @runtime.staticuint64s, i64

// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

func booliface() interface{} {
	// amd64:`LEAQ runtime.staticuint64s\+8\(SB\)`
	return true
}

func smallint8iface() interface{} {
	// amd64:`LEAQ runtime.staticuint64s\+2024\(SB\)`
	return int8(-3)
}

func smalluint8iface() interface{} {
	// amd64:`LEAQ runtime.staticuint64s\+24\(SB\)`
	return uint8(3)
}
