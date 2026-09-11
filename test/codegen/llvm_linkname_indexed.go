// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import (
	"go/types"
	_ "unsafe"
)

// Importing Checker gives this method an indexed package identity even though
// it is also referenced through a linkname. Preserve the native relocation.
//
//go:linkname llvmIndexedValidType go/types.(*Checker).validType
func llvmIndexedValidType(*types.Checker, *types.Named)

// LLVM: declare !goobj.import ![[IMPORT:[0-9]+]] goabiinternal void @"go/types.(*Checker).validType"(ptr, ptr)
// LLVM: ![[IMPORT]] = !{!"go/types", i32 {{[0-9]+}}, i32 0}
// LLVM-OPT: declare !goobj.import !{{[0-9]+}} goabiinternal void @"go/types.(*Checker).validType"(ptr, ptr)
// LLVM-NATIVE-OBJSUMMARY: NATIVE relocation {{.*}} target_kind=imported target_package="go/types" target_name="go/types.(*Checker).validType" target_index=[[INDEX:[0-9]+]]
// LLVM-NATIVE-OBJSUMMARY: LLVM relocation {{.*}} target_kind=imported target_package="go/types" target_name="go/types.(*Checker).validType" target_index=[[INDEX]]
func llvmIndexedLinkname(check *types.Checker, typ *types.Named) {
	llvmIndexedValidType(check, typ)
}
