// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

type llvmEmpty struct{}
type llvmZeroAlign8 [0]uint64

type llvmTrailingEmpty struct {
	Value byte
	Empty llvmEmpty
}

// LLVM-DAG: %go.abi.pad = type { i8 }
// LLVM-DAG: %codegen.llvmTrailingEmpty = type { i8, %codegen.llvmEmpty, %go.abi.pad }
// LLVM-DAG: define goabiinternal i8 @codegen.llvmZeroSizedArgs(i8 %a, { %codegen.llvmEmpty, %go.abi.pad } %e, { [0 x i64], %go.abi.pad } %z, i8 %b)
func llvmZeroSizedArgs(a byte, e llvmEmpty, z llvmZeroAlign8, b byte) byte {
	return a + b
}

// LLVM-DAG: define goabiinternal { i8, { %codegen.llvmEmpty, %go.abi.pad }, { [0 x i64], %go.abi.pad } } @codegen.llvmZeroSizedResults(i8 %value)
func llvmZeroSizedResults(value byte) (byte, llvmEmpty, llvmZeroAlign8) {
	return value, llvmEmpty{}, llvmZeroAlign8{}
}

// LLVM-DAG: define goabiinternal i8 @codegen.llvmTrailingZeroSizedField(%codegen.llvmTrailingEmpty %value)
func llvmTrailingZeroSizedField(value llvmTrailingEmpty) byte {
	return value.Value
}
