// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import "github.com/goallc/go-llvm"

func (lfc *LLVMFuncContext) simdMulWiden(v *Value, info goALLCSIMDOpInfo, laneType llvm.Type, lanes int) llvm.Value {
	x, y := lfc.simdLaneOperands(v, laneType, lanes)
	indices := make([]uint64, lanes/2)
	for i := range indices {
		indices[i] = uint64(i)
		if info.lowering == goALLCSIMDLowerMulWidenEven {
			indices[i] *= 2
		}
	}
	mask := llvmVectorShuffleMask(indices...)
	wideType := llvm.VectorType(GlobalCtxt.IntType(2*int(info.laneBits)), lanes/2)
	widen := func(x llvm.Value, name string) llvm.Value {
		x = lfc.b.CreateShuffleVector(x, x, mask, name+".selected")
		// Extend before multiplying: the full product, including its upper
		// half and sign, must survive even when it exceeds the input width.
		if info.lane == goALLCSIMDLaneInt {
			return lfc.b.CreateSExt(x, wideType, name+".wide")
		}
		return lfc.b.CreateZExt(x, wideType, name+".wide")
	}
	x, y = widen(x, v.String()+".x"), widen(y, v.String()+".y")
	return lfc.simdLaneResult(v, lfc.b.CreateMul(x, y, v.String()+".lanes"))
}
