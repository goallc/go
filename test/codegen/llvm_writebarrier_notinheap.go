// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build cgo

package codegen

import (
	"runtime/cgo"
	"unsafe"
)

type llvmRecordUnmanaged struct {
	_ cgo.Incomplete
}

var llvmRecordDestination unsafe.Pointer

// SSA may forward the integer carrier through the unsafe.Pointer conversion.
// The record must still use its fixed pointer signature, while the ordinary
// store retains the integer representation.
// LLVM: %[[PTR:[a-zA-Z0-9._]+]] = inttoptr i64 %p to ptr{{.*}}!goallc.notinheap
// LLVM: call void @goallc.gc.write.record(ptr %[[PTR]], ptr @codegen.llvmRecordDestination, i32 0)
// LLVM: store i64 %p, ptr @codegen.llvmRecordDestination
func llvmRecordUnmanagedPointer(p *llvmRecordUnmanaged) {
	llvmRecordDestination = unsafe.Pointer(p)
}

func llvmRecordHeapPointer(p *int) {
	llvmRecordDestination = unsafe.Pointer(p)
}
