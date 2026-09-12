// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import "github.com/goallc/go-llvm"

// simdImmediateShuffle lowers the constant SSA form, including forms reached
// through the frontend's existing immediate jump tables. AuxInt stores UInt8
// immediates sign-extended from int8; decode it before extracting fields.
func (lfc *LLVMFuncContext) simdImmediateShuffle(v *Value, info goALLCSIMDOpInfo, laneType llvm.Type, lanes int) llvm.Value {
	bits := int(info.laneBits)
	if !v.Type.IsSIMD() || int(v.Type.Size())*8 != lanes*bits || lanes*bits%128 != 0 {
		v.Fatalf("%s has invalid immediate shuffle shape", v.Op)
	}
	x := lfc.simdValueAs(v, v.Args[0], llvm.VectorType(laneType, lanes), ".x")
	y := llvm.ConstNull(x.Type())
	if len(v.Args) == 2 {
		y = lfc.simdValueAs(v, v.Args[1], x.Type(), ".y")
	}
	imm := uint8(v.AuxInt)
	group := 128 / bits
	indices := make([]uint64, lanes)
	keep := make([]llvm.Value, lanes)
	hasZero := false
	for i := range indices {
		base, local := i/group*group, i%group
		index, zero := i, false
		switch info.lowering {
		case goALLCSIMDLowerPermute32_128:
			index = base + int(imm>>uint(2*local)&3)
		case goALLCSIMDLowerPermuteLow16_128, goALLCSIMDLowerPermuteHigh16_128:
			offset := 0
			if info.lowering == goALLCSIMDLowerPermuteHigh16_128 {
				offset = 4
			}
			if local >= offset && local < offset+4 {
				index = base + offset + int(imm>>uint(2*(local-offset))&3)
			}
		case goALLCSIMDLowerConcatSelect128:
			// 32-bit controls repeat per 128-bit group. 64-bit controls
			// use a different pair of bits for each successive group.
			if bits == 32 {
				index = base + int(imm>>uint(2*local)&3)
			} else {
				index = base + int(imm>>uint(i)&1)
			}
			if local >= group/2 {
				index += lanes
			}
		case goALLCSIMDLowerConcatPermute128:
			control := imm >> uint(4*(i/group))
			index = int(control&3)*group + local
			zero = control&8 != 0
		case goALLCSIMDLowerConcatShiftBytes128:
			// The generic SSA inputs remain in API order: x is the high
			// half and y the low half of each concatenated byte group.
			offset := local + int(imm)
			index = lanes + base + offset
			if offset >= group {
				index = base + offset - group
			}
			zero = offset >= 2*group
		default:
			v.Fatalf("%s has unknown immediate shuffle recipe", v.Op)
		}
		if zero {
			// Never put an out-of-range or poison index into the shuffle.
			// A constant select supplies explicitly zeroed output lanes.
			index = 0
			hasZero = true
		}
		indices[i] = uint64(index)
		k := uint64(1)
		if zero {
			k = 0
		}
		keep[i] = llvm.ConstInt(GlobalCtxt.Int1Type(), k, false)
	}
	result := lfc.b.CreateShuffleVector(x, y, llvmVectorShuffleMask(indices...), v.String()+".shuffled")
	if hasZero {
		result = lfc.b.CreateSelect(llvm.ConstVector(keep, false), result, llvm.ConstNull(result.Type()), v.String()+".zeroed")
	}
	return lfc.simdLaneResult(v, result)
}
