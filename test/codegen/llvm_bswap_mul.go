// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "math/bits"

// LLVM-DAG: call i32 @llvm.bswap.i32(i32 %x)
func llvmBswap32(x uint32) uint32 {
	return bits.ReverseBytes32(x)
}

// LLVM-DAG: call i64 @llvm.bswap.i64(i64 %x)
func llvmBswap64(x uint64) uint64 {
	return bits.ReverseBytes64(x)
}

// LLVM-ARM64-DAG: call i32 @llvm.bitreverse.i32(i32 %x)
func llvmBitReverse32(x uint32) uint32 {
	return bits.Reverse32(x)
}

// LLVM-ARM64-DAG: call i64 @llvm.bitreverse.i64(i64 %x)
func llvmBitReverse64(x uint64) uint64 {
	return bits.Reverse64(x)
}

// LLVM-DAG: zext i64 %x to i128
// LLVM-DAG: zext i64 %y to i128
// LLVM-DAG: mul i128
// LLVM-DAG: lshr i128 {{%.*}}, 64
// LLVM-DAG: trunc i128 {{%.*}} to i64
func llvmMul64HiLo(x, y uint64) (high, low uint64) {
	return bits.Mul64(x, y)
}
