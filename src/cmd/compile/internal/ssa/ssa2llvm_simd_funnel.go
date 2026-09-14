// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"fmt"

	"github.com/goallc/go-llvm"
)

func (lfc *LLVMFuncContext) simdFunnelShift(v *Value, info goALLCSIMDOpInfo, laneType llvm.Type, lanes int) llvm.Value {
	vectorType := llvm.VectorType(laneType, lanes)
	x := lfc.simdValueAs(v, v.Args[0], vectorType, ".x")
	y := x
	var count llvm.Value
	switch info.lowering {
	case goALLCSIMDLowerRotateLeft, goALLCSIMDLowerRotateRight:
		count = lfc.simdValueAs(v, v.Args[1], vectorType, ".count")
	case goALLCSIMDLowerFunnelAllLeft, goALLCSIMDLowerFunnelAllRight:
		y = lfc.simdValueAs(v, v.Args[1], vectorType, ".y")
		// The existing frontend immediate expansion supplies UInt8 AuxInt,
		// including when the public uint64 count is not a constant.
		count = llvmSIMDIntegerSplat(laneType, lanes, uint64(uint8(v.AuxInt)))
	default:
		y = lfc.simdValueAs(v, v.Args[1], vectorType, ".y")
		count = lfc.simdValueAs(v, v.Args[2], vectorType, ".count")
	}
	op := "fshl"
	switch info.lowering {
	case goALLCSIMDLowerRotateRight, goALLCSIMDLowerFunnelRight, goALLCSIMDLowerFunnelAllRight:
		op = "fshr"
		// Go's right concatenation is {y:x}; LLVM extracts the low half.
		x, y = y, x
	}
	// Funnel intrinsics define all counts modulo the lane width, including
	// zero and negative-looking bit patterns. No poison-producing shifts.
	sig := llvm.FunctionType(vectorType, []llvm.Type{vectorType, vectorType, vectorType}, false)
	fn := getOrInsertLLVMIntrinsic(fmt.Sprintf("llvm.%s.v%di%d", op, lanes, info.laneBits), sig)
	return lfc.simdLaneResult(v, lfc.b.CreateCall(sig, fn, []llvm.Value{x, y, count}, v.String()))
}
