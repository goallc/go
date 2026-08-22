// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "internal/runtime/sys"

// LLVM-LABEL: define goabiinternal i64 @codegen.llvmCallerSP()
// LLVM-SAME: #[[SPATTR:[0-9]+]] gc "goallc"
// LLVM-ARM64: call ptr @llvm.sponentry.p0()
// LLVM-AMD64: call ptr @llvm.addressofreturnaddress.p0()
// LLVM-AMD64: getelementptr i8, ptr {{%.*}}, i64 8
// LLVM: call i64 @llvm.go.pointer.address.i64.p0(ptr
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmCallerPC()
// LLVM-SAME: #[[SPATTR]] gc "goallc"
// LLVM: call ptr @llvm.returnaddress.p0(i32 0)
// LLVM: ptrtoint ptr {{%.*}} to i64
// LLVM-DAG: attributes #[[SPATTR]] = { {{.*}}noinline
func llvmCallerPC() uintptr {
	return sys.GetCallerPC()
}

func llvmCallerSP() uintptr {
	return sys.GetCallerSP()
}
