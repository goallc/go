// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "unsafe"

// Functions are emitted in reverse declaration order.
//
// LLVM-LABEL: define goabiinternal i64 @codegen.goPointerAddress(
// LLVM: call i64 @llvm.go.pointer.address.i64.p0(ptr %pointer)
// LLVM-LABEL: define goabiinternal ptr @"codegen.(*goABIEntryReceiver).entryArgs"(
// LLVM-NOT: llvm.experimental.stackmap
// LLVM-LABEL: define goabiinternal ptr @codegen.goABIEntryArgs(
// LLVM-SAME: ptr %pointer, i64 %scalar)
// LLVM-NOT: llvm.experimental.stackmap
// LLVM-LABEL: define goabiinternal i64 @codegen.goABIStatepointAttributes(
// LLVM-SAME: i64 %x) #[[ATTRS:[0-9]+]] gc "goallc" {{.*}}{
// LLVM: attributes #[[ATTRS]] = { {{.*}}"frame-pointer"="non-leaf"{{.*}} }
// LLVM-NOT: go-stack-growth-statepoint
func goABIStatepointAttributes(x int) int {
	return x + 1
}

func goABIEntryArgs(pointer *int, scalar int) *int {
	return pointer
}

type goABIEntryReceiver struct{}

func (receiver *goABIEntryReceiver) entryArgs(scalar int) *goABIEntryReceiver {
	return receiver
}

func goPointerAddress(pointer unsafe.Pointer) uintptr {
	return uintptr(pointer)
}
