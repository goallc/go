// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import "github.com/goallc/go-llvm"

func (lfc *LLVMFuncContext) simdDotPairs(v *Value, laneType llvm.Type, lanes int) llvm.Value {
	x, y := lfc.simdLaneOperands(v, laneType, lanes)
	wideType := llvm.VectorType(GlobalCtxt.Int32Type(), lanes)
	x = lfc.b.CreateSExt(x, wideType, v.String()+".x")
	y = lfc.b.CreateSExt(y, wideType, v.String()+".y")
	product := lfc.b.CreateMul(x, y, v.String()+".product")
	even, odd := make([]uint64, lanes/2), make([]uint64, lanes/2)
	for i := range even {
		even[i], odd[i] = uint64(2*i), uint64(2*i+1)
	}
	a := lfc.b.CreateShuffleVector(product, product, llvmVectorShuffleMask(even...), v.String()+".even")
	b := lfc.b.CreateShuffleVector(product, product, llvmVectorShuffleMask(odd...), v.String()+".odd")
	// In particular, two (-32768 * -32768) products wrap to -2147483648.
	return lfc.simdLaneResult(v, lfc.b.CreateAdd(a, b, v.String()))
}

func (lfc *LLVMFuncContext) simdGroupedByteArithmetic(v *Value, info goALLCSIMDOpInfo, laneType llvm.Type, lanes int) llvm.Value {
	x, y := lfc.simdLaneOperands(v, laneType, lanes)
	// Generic expansion misses the single-instruction idioms in the pinned
	// LLVM. Use its existing intrinsics; CPU requirements still come from Go.
	names := [...]string{"llvm.x86.sse2.psad.bw", "llvm.x86.avx2.psad.bw", "llvm.x86.avx512.psad.bw.512"}
	resultBits := 64
	if info.lowering == goALLCSIMDLowerDotPairsUSSat {
		names = [...]string{"llvm.x86.ssse3.pmadd.ub.sw.128", "llvm.x86.avx2.pmadd.ub.sw", "llvm.x86.avx512.pmaddubs.w.512"}
		resultBits = 16
	}
	index := 0
	if lanes == 32 {
		index = 1
	} else if lanes == 64 {
		index = 2
	}
	resultType := llvm.VectorType(GlobalCtxt.IntType(resultBits), lanes*8/resultBits)
	sig := llvm.FunctionType(resultType, []llvm.Type{x.Type(), y.Type()}, false)
	fn := getOrInsertLLVMIntrinsic(names[index], sig)
	return lfc.simdLaneResult(v, lfc.b.CreateCall(sig, fn, []llvm.Value{x, y}, v.String()))
}
