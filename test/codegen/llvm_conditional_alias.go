// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "unsafe"

//go:linkname llvmScalar16 runtime.convT16
func llvmScalar16(uint16) unsafe.Pointer

//go:linkname llvmScalar32 runtime.convT32
func llvmScalar32(uint32) unsafe.Pointer

//go:linkname llvmScalar64 runtime.convT64
func llvmScalar64(uint64) unsafe.Pointer

func llvmFreshScalar16() unsafe.Pointer           { return llvmScalar16(256) }
func llvmFreshScalar32() unsafe.Pointer           { return llvmScalar32(256) }
func llvmFreshScalar64() unsafe.Pointer           { return llvmScalar64(256) }
func llvmCachedScalar64() unsafe.Pointer          { return llvmScalar64(255) }
func llvmDynamicScalar64(n uint64) unsafe.Pointer { return llvmScalar64(n) }
func llvmFreshMap(n int) map[int]int              { return make(map[int]int, n) }
func llvmFreshSlice(n int) []uint64               { return make([]uint64, n, 4) }
func llvmCopySlice(src []uint64) []uint64         { dst := make([]uint64, 4); copy(dst, src); return dst }
func llvmFreshStringBox(s string) any             { return s[:3] }

var llvmBoxArray [8]byte

func llvmFreshSliceBox() any { return llvmBoxArray[:0] }

// LLVM-DAG: call goabiinternal noalias ptr @"runtime.convT16<linkname>"(i16 256)
// LLVM-DAG: call goabiinternal noalias ptr @"runtime.convT32<linkname>"(i32 256)
// LLVM-DAG: call goabiinternal noalias ptr @"runtime.convT64<linkname>"(i64 256)
// LLVM-DAG: call goabiinternal ptr @"runtime.convT64<linkname>"(i64 255)
// LLVM-DAG: call goabiinternal ptr @"runtime.convT64<linkname>"(i64 %n)
// LLVM-DAG: call goabiinternal noalias ptr @"runtime.makemap<builtin.{{[0-9]+}}>"
// LLVM-DAG: call goabiinternal noalias ptr @"runtime.makeslice<builtin.{{[0-9]+}}>"
// LLVM-DAG: call goabiinternal noalias ptr @"runtime.convTstring<builtin.{{[0-9]+}}>"
// LLVM-DAG: call goabiinternal noalias ptr @"runtime.convTslice<builtin.{{[0-9]+}}>"
