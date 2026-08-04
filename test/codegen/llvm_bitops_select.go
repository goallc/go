// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import (
	"crypto/subtle"
	"math/bits"
)

// LLVM-DAG: define goabiinternal i64 @codegen.llvmRotateLeft64(i64 %x, i64 %count)
// LLVM-DAG: call i64 @llvm.fshl.i64(i64 %x, i64 %x, i64 %count)
func llvmRotateLeft64(x uint64, count int) uint64 {
	return bits.RotateLeft64(x, count)
}

// LLVM-DAG: define goabiinternal i32 @codegen.llvmRotateLeft32(i32 %x, i64 %count)
// LLVM-DAG: trunc i64 %count to i32
// LLVM-DAG: call i32 @llvm.fshl.i32(i32 %x, i32 %x, i32 %{{.*}})
func llvmRotateLeft32(x uint32, count int) uint32 {
	return bits.RotateLeft32(x, count)
}

// LLVM-DAG: define goabiinternal i64 @codegen.llvmBitLen64(i64 %x)
// LLVM-DAG: call i64 @llvm.ctlz.i64(i64 %x, i1 false)
// LLVM-DAG: sub i64 64,
func llvmBitLen64(x uint64) int {
	return bits.Len64(x)
}

// LLVM-DAG: define goabiinternal i64 @codegen.llvmBitLen32(i32 %x)
// LLVM-DAG: call i32 @llvm.ctlz.i32(i32 %x, i1 false)
// LLVM-DAG: sub i32 32,
// LLVM-DAG: zext i32 %{{.*}} to i64
func llvmBitLen32(x uint32) int {
	return bits.Len32(x)
}

// LLVM-DAG: define goabiinternal i64 @codegen.llvmTrailingZeros64(i64 %x)
// LLVM-DAG: call i64 @llvm.cttz.i64(i64 %x, i1 false)
func llvmTrailingZeros64(x uint64) int {
	return bits.TrailingZeros64(x)
}

// LLVM-DAG: define goabiinternal i64 @codegen.llvmTrailingZeros32(i32 %x)
// LLVM-DAG: call i32 @llvm.cttz.i32(i32 %x, i1 false)
// LLVM-DAG: zext i32 %{{.*}} to i64
func llvmTrailingZeros32(x uint32) int {
	return bits.TrailingZeros32(x)
}

// LLVM-DAG: define goabiinternal i64 @codegen.llvmTrailingZeros16(i16 %x)
// LLVM-DAG: call i16 @llvm.cttz.i16(i16 %x, i1 false)
// LLVM-DAG: zext i16 %{{.*}} to i64
func llvmTrailingZeros16(x uint16) int {
	return bits.TrailingZeros16(x)
}

// LLVM-DAG: define goabiinternal i64 @codegen.llvmTrailingZeros8(i8 %x)
// LLVM-DAG: call i8 @llvm.cttz.i8(i8 %x, i1 false)
// LLVM-DAG: zext i8 %{{.*}} to i64
func llvmTrailingZeros8(x uint8) int {
	return bits.TrailingZeros8(x)
}

// LLVM-DAG: define goabiinternal i64 @codegen.llvmTrailingZeros64NonZero(i64 %x)
// LLVM-DAG: call i64 @llvm.cttz.i64(i64 %x, i1 true)
func llvmTrailingZeros64NonZero(x uint64) int {
	if x == 0 {
		return -1
	}
	return bits.TrailingZeros64(x)
}

// LLVM-DAG: define goabiinternal i64 @codegen.llvmPopulationCount64(i64 %x)
// LLVM-DAG: call i64 @llvm.ctpop.i64(i64 %x)
func llvmPopulationCount64(x uint64) int {
	return bits.OnesCount64(x)
}

// LLVM-DAG: define goabiinternal i64 @codegen.llvmPopulationCount16(i16 %x)
// LLVM-DAG: call i16 @llvm.ctpop.i16(i16 %x)
// LLVM-DAG: zext i16 %{{.*}} to i64
func llvmPopulationCount16(x uint16) int {
	return bits.OnesCount16(x)
}

// LLVM-DAG: define goabiinternal i64 @codegen.llvmCondSelect(i64 %{{.*}}, i64 %{{.*}}, i64 %{{.*}})
// LLVM-DAG: select i1 %{{.*}}, i64 %{{.*}}, i64 %{{.*}}
func llvmCondSelect(cond, x, y int) int {
	return subtle.ConstantTimeSelect(cond, x, y)
}
