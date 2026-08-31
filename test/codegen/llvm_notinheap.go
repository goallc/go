// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build cgo

package codegen

import (
	"runtime"
	"runtime/cgo"
	"unsafe"
)

type llvmNotInHeap struct {
	_     cgo.Incomplete
	value uintptr
}

//go:noinline
func llvmNotInHeapCall() { runtime.Gosched() }

// A pointer to unmanaged storage must remain an integer across the call. Its
// terminal dereference uses a plain inttoptr so statepoint rewriting may
// rematerialize it without treating the unmanaged address as a GC root.
//
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmNotInHeapAcrossCall(i64 %p)
// LLVM: call goabiinternal void @codegen.llvmNotInHeapCall()
// LLVM-NOT: llvm.go.pointer.from.address
// LLVM: inttoptr i64 %p to ptr{{.*}}!goallc.notinheap
// LLVM: load i64
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmNotInHeapAcrossCall(i64 %p)
// LLVM-OPT: call goabiinternal void @codegen.llvmNotInHeapCall()
// LLVM-OPT-NOT: llvm.go.pointer.from.address
// LLVM-OPT: inttoptr i64 %p to ptr{{.*}}!goallc.notinheap
// LLVM-OPT: load i64
func llvmNotInHeapAcrossCall(p *llvmNotInHeap) uintptr {
	llvmNotInHeapCall()
	return p.value
}

// Slices backed by unmanaged storage use the same integer data-word carrier.
//
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmNotInHeapSliceData({ i64, i64, i64 } %s)
// LLVM: extractvalue { i64, i64, i64 } %s, 0
// LLVM: ret i64
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmNotInHeapSliceData({ i64, i64, i64 } %s)
// LLVM-OPT: extractvalue { i64, i64, i64 } %s, 0
// LLVM-OPT: ret i64
func llvmNotInHeapSliceData(s []llvmNotInHeap) *llvmNotInHeap {
	return unsafe.SliceData(s)
}
