// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "runtime"

// LLVM-DAG: @codegen..inittask = global <{ [8 x i8], ptr }> <{ [8 x i8] c"\00\00\00\00\01\00\00\00", ptr @codegen.init.0 }>, section ".noptrdata"
// LLVM-DAG: !goobj.marker_relocs
// LLVM-DAG: !{ptr @codegen..inittask, ptr @runtime..inittask, i32 102, i64 0}
// LLVM-OBJSUMMARY: LLVM relocation owner="codegen..inittask" type=R_INITORDER size=0 target_kind=none target_package="" target_name="" target_index={{[1-9][0-9]*}}

var llvmInitTaskValue int

func init() {
	runtime.Gosched()
	llvmInitTaskValue = 42
}
