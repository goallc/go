// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-LABEL: define goabiinternal void @codegen.llvmNewproc(
// LLVM: call goabiinternal void @runtime.newproc(ptr
// LLVM-NOT: call goabiinternal void @runtime.newproc(i64
// LLVM: ret void
// LLVM: declare goabiinternal void @runtime.newproc(ptr)
var llvmNewprocSink int

func llvmNewproc(value int) {
	go func() { llvmNewprocSink = value }()
}
