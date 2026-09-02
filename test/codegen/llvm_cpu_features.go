// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import (
	"math"
	"math/bits"
	"sync/atomic"
)

var llvmCodegenCPUCounter uint64

// LLVM-NM-AMD64: codegen.llvmCodegenCPUBits.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-3: codegen.llvmCodegenCPUBits<1>
// LLVM-NM-AMD64-NOT: codegen.llvmCodegenCPUBits<1>
// LLVM-NM-AMD64: codegen.llvmCodegenCPUMath.goallc.fmv.slot
// LLVM-NM-AMD64-COUNT-5: codegen.llvmCodegenCPUMath<1>
// LLVM-NM-AMD64-NOT: codegen.llvmCodegenCPUMath<1>
// LLVM-NM-AMD64-NOT: <goallc.fmv.
// LLVM-ASM-AMD64-DAG: ROUNDSD
// LLVM-ASM-AMD64-DAG: VFMADD
//
//go:noinline
func llvmCodegenCPUMath(x, y, z float64) (float64, float64) {
	return math.Floor(x), math.FMA(x, y, z)
}

// LLVM-ASM-AMD64: POPCNTQ
//
//go:noinline
func llvmCodegenCPUBits(x uint64) int {
	return bits.OnesCount64(x)
}

// LLVM-NM-ARM64: codegen.llvmCodegenCPUAtomic.goallc.fmv.slot
// LLVM-NM-ARM64-COUNT-3: codegen.llvmCodegenCPUAtomic<1>
// LLVM-NM-ARM64-NOT: codegen.llvmCodegenCPUAtomic<1>
// LLVM-NM-ARM64-NOT: <goallc.fmv.
//
//go:noinline
func llvmCodegenCPUAtomic(delta uint64) uint64 {
	return atomic.AddUint64(&llvmCodegenCPUCounter, delta)
}
