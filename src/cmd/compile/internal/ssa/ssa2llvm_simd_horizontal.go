// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"fmt"
	"github.com/goallc/go-llvm"
)

// simdHorizontal routes adjacent pairs before applying lane arithmetic.
// Grouped x86 operations concatenate results within each 128-bit group.
func (lfc *LLVMFuncContext) simdHorizontal(v *Value, info goALLCSIMDOpInfo, laneType llvm.Type, lanes int) llvm.Value {
	group := lanes
	switch info.lowering {
	case goALLCSIMDLowerPairAdd128, goALLCSIMDLowerPairSub128, goALLCSIMDLowerPairSAddSat128, goALLCSIMDLowerPairSSubSat128:
		group = 128 / int(info.laneBits)
	}
	subtract, saturated := false, false
	switch info.lowering {
	case goALLCSIMDLowerPairSub, goALLCSIMDLowerPairSub128, goALLCSIMDLowerPairSSubSat, goALLCSIMDLowerPairSSubSat128:
		subtract = true
	}
	switch info.lowering {
	case goALLCSIMDLowerPairSAddSat, goALLCSIMDLowerPairSAddSat128, goALLCSIMDLowerPairSSubSat, goALLCSIMDLowerPairSSubSat128:
		saturated = true
	}
	x, y := lfc.simdLaneOperands(v, laneType, lanes)
	even, odd := make([]uint64, lanes), make([]uint64, lanes)
	for i := range even {
		j := (i/group)*group + 2*(i%(group/2))
		if i%group >= group/2 {
			j += lanes
		}
		even[i], odd[i] = uint64(j), uint64(j+1)
	}
	a := lfc.b.CreateShuffleVector(x, y, llvmVectorShuffleMask(even...), v.String()+".even")
	b := lfc.b.CreateShuffleVector(x, y, llvmVectorShuffleMask(odd...), v.String()+".odd")
	build := lfc.b.CreateAdd
	if subtract {
		build = lfc.b.CreateSub
	}
	if info.lane == goALLCSIMDLaneFloat {
		build = lfc.b.CreateFAdd
		if subtract {
			build = lfc.b.CreateFSub
		}
	}
	if saturated {
		op := "sadd"
		if subtract {
			op = "ssub"
		}
		sig := llvm.FunctionType(a.Type(), []llvm.Type{a.Type(), a.Type()}, false)
		fn := getOrInsertLLVMIntrinsic(fmt.Sprintf("llvm.%s.sat.v%di%d", op, lanes, info.laneBits), sig)
		return lfc.simdLaneResult(v, lfc.b.CreateCall(sig, fn, []llvm.Value{a, b}, v.String()))
	}
	return lfc.simdLaneResult(v, build(a, b, v.String()))
}
