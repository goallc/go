// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

type llvmDirectIfaceLeaf struct {
	ptr *int
}

type llvmDirectIfaceNested struct {
	_     struct{}
	_     [2][0]uint64
	value [1]llvmDirectIfaceLeaf
}

//go:noinline
func llvmDirectIfaceSink(llvmDirectIfaceNested) {}

// Extracting the data word from a constant interface may fold directly to the
// referenced Go global. It must retain that global's symbol name.
//
// LLVM-LABEL: define goabiinternal void @codegen.llvmIDataConstant(
// LLVM: store ptr @runtime.zeroVal

// A direct-interface aggregate is physically one pointer in the interface
// data word. Rebuild the logical nested aggregate before passing it through
// the ordinary ABI call path.
//
// LLVM-LABEL: define goabiinternal void @codegen.llvmDirectIfaceCall(
// LLVM: [[SLOT:%.*]] = alloca %codegen.llvmDirectIfaceNested, align 8
// LLVM: [[DATA:%.*]] = extractvalue { i64, ptr } %x, 1
// LLVM: [[LEAF:%.*]] = insertvalue %codegen.llvmDirectIfaceLeaf undef, ptr [[DATA]], 0
// LLVM: [[ARRAY:%.*]] = insertvalue [1 x %codegen.llvmDirectIfaceLeaf] undef, %codegen.llvmDirectIfaceLeaf [[LEAF]], 0
// LLVM: [[NESTED:%.*]] = insertvalue %codegen.llvmDirectIfaceNested {{.*}}, [1 x %codegen.llvmDirectIfaceLeaf] [[ARRAY]], 2
// LLVM: store %codegen.llvmDirectIfaceNested [[NESTED]], ptr [[SLOT]], align 8
// LLVM: call goabiinternal void @codegen.llvmDirectIfaceSink(ptr byval(%codegen.llvmDirectIfaceNested) align 8 [[SLOT]])
func llvmDirectIfaceCall(x any) {
	switch x := x.(type) {
	case llvmDirectIfaceNested:
		llvmDirectIfaceSink(x)
	}
}

type llvmIDataZero struct {
	ok    bool
	value int
}

//go:noinline
func llvmIDataSink(...any) {}

func llvmIDataConstant() {
	llvmIDataSink(llvmIDataZero{})
}
