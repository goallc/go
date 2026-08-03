// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "reflect"

// LLVM-LABEL: define goabiinternal %reflect.Value @codegen.llvmReflectMethod(
// LLVM-SAME: !goobj.symbol.flags ![[FLAGS:[0-9]+]]
// LLVM-DAG: ![[FLAGS]] = !{i32 32, i32 0}
func llvmReflectMethod(v reflect.Value) reflect.Value {
	return v.Method(0)
}
