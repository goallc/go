// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.simd && amd64

package codegen

import "simd/archsimd"

// The returned pointer keeps the heap allocation observable. Its size class
// aligns the vector store more strongly than the Go array element type does.
func llvmAllocationStore128(v archsimd.Uint64x2) *[16]uint64 {
	p := new([16]uint64)
	v.Store(p[:2])
	return p
}

// A 24-byte slot only guarantees 8-byte alignment. Do not select an aligned
// 16-byte store just because the object is larger than the vector.
func llvmAllocationStore24(v archsimd.Uint64x2) *[3]uint64 {
	p := new([3]uint64)
	v.Store(p[:2])
	return p
}

// LLVM-OPT-DAG: store <2 x i64> {{.*}}, ptr {{.*}}, align 128,
// LLVM-OPT-DAG: store <2 x i64> {{.*}}, ptr {{.*}}, align 8,
