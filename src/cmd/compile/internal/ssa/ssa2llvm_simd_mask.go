// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import "github.com/goallc/go-llvm"

func (lfc *LLVMFuncContext) simdMasked(v *Value, info goALLCSIMDOpInfo, laneType llvm.Type, lanes int) llvm.Value {
	x := lfc.simdValueAs(v, v.Args[0], llvm.VectorType(laneType, lanes), ".x")
	maskType := llvm.VectorType(GlobalCtxt.IntType(int(info.laneBits)), lanes)
	mask := lfc.simdValueAs(v, v.Args[len(v.Args)-1], maskType, ".mask")
	// Generic SSA carries vector masks. Match the native VPMOVVecToM
	// conversion (or VPBLENDVB): only each lane's sign bit is significant.
	condition := lfc.b.CreateICmp(llvm.IntSLT, mask, llvm.ConstNull(maskType), v.String()+".condition")
	var result llvm.Value
	switch info.lowering {
	case goALLCSIMDLowerCompress, goALLCSIMDLowerExpand:
		name := "llvm.x86.avx512.mask.compress"
		if info.lowering == goALLCSIMDLowerExpand {
			name = "llvm.x86.avx512.mask.expand"
		}
		fn := getLLVMIntrinsicDeclaration(name, x.Type())
		result = lfc.b.CreateCall(fn.GlobalValueType(), fn, []llvm.Value{x, llvm.ConstNull(x.Type()), condition}, v.String()+".masked")
	case goALLCSIMDLowerBlendMasked, goALLCSIMDLowerBlendBytes:
		y := lfc.simdValueAs(v, v.Args[1], x.Type(), ".y")
		result = lfc.b.CreateSelect(condition, y, x, v.String()+".masked")
	case goALLCSIMDLowerBroadcastLowMasked:
		resultLanes := int(v.Type.Size()) * 8 / int(info.laneBits)
		x = lfc.b.CreateShuffleVector(x, llvm.ConstNull(x.Type()), llvmVectorShuffleMask(make([]uint64, resultLanes)...), v.String()+".broadcast")
		if resultLanes != lanes {
			// The upstream private API supplies a source-width mask. Native
			// conversion zeros the remaining K bits, not repeats them.
			indices := make([]uint64, resultLanes)
			for i := range indices {
				indices[i] = uint64(min(i, lanes))
			}
			condition = lfc.b.CreateShuffleVector(condition, llvm.ConstNull(condition.Type()), llvmVectorShuffleMask(indices...), v.String()+".wide-mask")
		}
		result = lfc.b.CreateSelect(condition, x, llvm.ConstNull(x.Type()), v.String()+".masked")
	}
	return lfc.simdLaneResult(v, result)
}
