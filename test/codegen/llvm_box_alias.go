// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "unsafe"

type llvmBoxWords struct{ a, b, c uintptr }
type llvmBoxPointer struct {
	p *int
	n int
}

func llvmBoxWordsValue(a, b, c uintptr) any { return llvmBoxWords{a, b, c} }
func llvmBoxPointerValue(p *int, n int) any { return llvmBoxPointer{p, n} }
func llvmMakeSmallMap() map[int]int         { return make(map[int]int) }

//go:linkname llvmBoxDynamic runtime.convT
func llvmBoxDynamic(typ, src unsafe.Pointer) unsafe.Pointer
func llvmBoxUnknownType(typ, src unsafe.Pointer) unsafe.Pointer { return llvmBoxDynamic(typ, src) }

// LLVM-DAG: call goabiinternal noalias ptr @"runtime.convTnoptr<builtin.{{[0-9]+}}>"
// LLVM-DAG: call goabiinternal noalias ptr @"runtime.convT<builtin.{{[0-9]+}}>"(ptr @"type:codegen.llvmBoxPointer"
// LLVM-DAG: call goabiinternal ptr @"runtime.convT<builtin.{{[0-9]+}}>"(ptr %typ, ptr %src)
// LLVM-DAG: declare goabiinternal noalias nonnull ptr @"runtime.makemap_small<builtin.{{[0-9]+}}>"()
// LLVM-DAG: declare goabiinternal nonnull ptr @"runtime.convT<builtin.{{[0-9]+}}>"
// LLVM-DAG: declare goabiinternal nonnull ptr @"runtime.convTnoptr<builtin.{{[0-9]+}}>"
