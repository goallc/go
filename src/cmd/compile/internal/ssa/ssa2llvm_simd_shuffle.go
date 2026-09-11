// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import "github.com/goallc/go-llvm"

// simdStaticShuffle expresses fixed lane routing as standard LLVM IR. The
// generated recipe describes semantics; register width and indices come from
// the SSA operand/result shapes, including native width-only carriers.
func (lfc *LLVMFuncContext) simdStaticShuffle(v *Value, info goALLCSIMDOpInfo, laneType llvm.Type, lanes int) llvm.Value {
	if !v.Type.IsSIMD() || lanes < 2 || lanes%2 != 0 {
		v.Fatalf("%s has invalid static shuffle shape", v.Op)
	}
	resultLanes := int(v.Type.Size()) * 8 / int(info.laneBits)
	x := lfc.simdValueAs(v, v.Args[0], llvm.VectorType(laneType, lanes), ".x")
	y := llvm.ConstNull(x.Type())
	indices := make([]uint64, resultLanes)
	switch info.lowering {
	case goALLCSIMDLowerGetLow, goALLCSIMDLowerGetHigh:
		if len(v.Args) != 1 || resultLanes*2 != lanes {
			v.Fatalf("%s has invalid half extraction shape", v.Op)
		}
		offset := 0
		if info.lowering == goALLCSIMDLowerGetHigh {
			offset = resultLanes
		}
		for i := range indices {
			indices[i] = uint64(offset + i)
		}
	case goALLCSIMDLowerSetLow, goALLCSIMDLowerSetHigh:
		if len(v.Args) != 2 || resultLanes != lanes || v.Args[1].Type.Size()*2 != v.Type.Size() {
			v.Fatalf("%s has invalid half insertion shape", v.Op)
		}
		half := lanes / 2
		y = lfc.simdValueAs(v, v.Args[1], llvm.VectorType(laneType, half), ".y")
		// Both shuffle operands must have equal lane counts. Pad y explicitly
		// with zeros; none of these padding lanes is selected in the result.
		pad := make([]uint64, lanes)
		for i := range pad {
			pad[i] = uint64(i)
		}
		y = lfc.b.CreateShuffleVector(y, llvm.ConstNull(y.Type()), llvmVectorShuffleMask(pad...), v.String()+".padded")
		offset := 0
		if info.lowering == goALLCSIMDLowerSetHigh {
			offset = half
		}
		for i := range indices {
			indices[i] = uint64(i)
			if i >= offset && i < offset+half {
				indices[i] = uint64(lanes + i - offset)
			}
		}
	case goALLCSIMDLowerBroadcastLow:
		if len(v.Args) != 1 || lanes*int(info.laneBits) != 128 || resultLanes < lanes {
			v.Fatalf("%s has invalid broadcast shape", v.Op)
		}
		// The zero-valued mask repeats source lane zero at any result width.
	default:
		if len(v.Args) != 2 || resultLanes != lanes {
			v.Fatalf("%s has invalid two-input shuffle shape", v.Op)
		}
		y = lfc.simdValueAs(v, v.Args[1], x.Type(), ".y")
		group, offset := lanes, 0
		switch info.lowering {
		case goALLCSIMDLowerInterleaveLow128, goALLCSIMDLowerInterleaveHigh128:
			group = 128 / int(info.laneBits)
		}
		switch info.lowering {
		case goALLCSIMDLowerInterleaveHigh, goALLCSIMDLowerInterleaveHigh128:
			offset = group / 2
		case goALLCSIMDLowerConcatOdd, goALLCSIMDLowerInterleaveOdd:
			offset = 1
		}
		for i := range indices {
			switch info.lowering {
			case goALLCSIMDLowerConcatEven, goALLCSIMDLowerConcatOdd:
				indices[i] = uint64(2*i + offset)
			case goALLCSIMDLowerInterleaveEven, goALLCSIMDLowerInterleaveOdd:
				indices[i] = uint64(2*(i/2) + offset + (i%2)*lanes)
			case goALLCSIMDLowerInterleaveLow, goALLCSIMDLowerInterleaveHigh,
				goALLCSIMDLowerInterleaveLow128, goALLCSIMDLowerInterleaveHigh128:
				indices[i] = uint64((i/group)*group + (i%group)/2 + offset + (i%2)*lanes)
			default:
				v.Fatalf("%s has unknown static shuffle recipe", v.Op)
			}
		}
	}
	return lfc.simdLaneResult(v, lfc.b.CreateShuffleVector(x, y, llvmVectorShuffleMask(indices...), v.String()+".shuffled"))
}
