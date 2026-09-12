// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"fmt"
	"github.com/goallc/go-llvm"
)

// Constant indices fold through standard IR. Dynamic indices use existing LLVM
// target intrinsics: LLVM currently scalarizes an extract/insert expansion,
// including spilling the source vector for variable-index stack loads.
func (lfc *LLVMFuncContext) simdDynamicShuffle(v *Value, info goALLCSIMDOpInfo, laneType llvm.Type, lanes int) llvm.Value {
	vectorType := llvm.VectorType(laneType, lanes)
	indexType := GlobalCtxt.IntType(int(info.laneBits))
	data, indexArg := 0, len(v.Args)-1
	if info.lowering == goALLCSIMDLowerPermute || info.lowering == goALLCSIMDLowerConcatPermute {
		data, indexArg = 1, 0
	}
	x := lfc.simdValueAs(v, v.Args[data], vectorType, ".x")
	indices := lfc.simdValueAs(v, v.Args[indexArg], llvm.VectorType(indexType, lanes), ".indices")
	group := lanes
	if info.lowering == goALLCSIMDLowerPermuteOrZero128 {
		group = 128 / int(info.laneBits)
	}
	y := llvm.Value{}
	if info.lowering == goALLCSIMDLowerConcatPermute {
		y = lfc.simdValueAs(v, v.Args[2], vectorType, ".y")
	}
	if indices.IsAConstant().IsNil() {
		return lfc.simdDynamicShuffleTarget(v, info, x, y, indices, lanes)
	}
	// Mask even indices whose output is subsequently zeroed, so neither
	// constant folding nor later optimization can introduce poison.
	constant := func(n int) llvm.Value { return llvm.ConstInt(indexType, uint64(n), false) }
	result := llvm.ConstNull(vectorType)
	for i := 0; i < lanes; i++ {
		position := llvm.ConstInt(GlobalCtxt.Int32Type(), uint64(i), false)
		index := lfc.b.CreateExtractElement(indices, position, "")
		safe := lfc.b.CreateAnd(index, constant(group-1), "")
		if group != lanes {
			safe = lfc.b.CreateAdd(safe, constant(i/group*group), "")
		}
		element := lfc.b.CreateExtractElement(x, safe, "")
		switch info.lowering {
		case goALLCSIMDLowerConcatPermute:
			other := lfc.b.CreateExtractElement(y, safe, "")
			upper := lfc.b.CreateICmp(llvm.IntNE, lfc.b.CreateAnd(index, constant(lanes), ""), constant(0), "")
			element = lfc.b.CreateSelect(upper, other, element, "")
		case goALLCSIMDLowerLookupOrZero:
			// Unsigned comparison also rejects negative signed byte indices.
			valid := lfc.b.CreateICmp(llvm.IntULT, index, constant(lanes), "")
			element = lfc.b.CreateSelect(valid, element, llvm.ConstNull(laneType), "")
		case goALLCSIMDLowerPermuteOrZero, goALLCSIMDLowerPermuteOrZero128:
			valid := lfc.b.CreateICmp(llvm.IntSGE, index, constant(0), "")
			element = lfc.b.CreateSelect(valid, element, llvm.ConstNull(laneType), "")
		}
		result = lfc.b.CreateInsertElement(result, element, position, "")
	}
	return lfc.simdLaneResult(v, result)
}

func (lfc *LLVMFuncContext) simdShuffleCall(v *Value, name string, args ...llvm.Value) llvm.Value {
	argTypes := make([]llvm.Type, len(args))
	for i, arg := range args {
		argTypes[i] = arg.Type()
	}
	sig := llvm.FunctionType(args[0].Type(), argTypes, false)
	var fn llvm.Value
	if name == "llvm.aarch64.neon.tbl1" {
		fn = getLLVMIntrinsicDeclaration(name, args[0].Type())
	} else {
		fn = getLLVMIntrinsicDeclaration(name)
	}
	if fn.GlobalValueType() != sig {
		v.Fatalf("%s has unexpected intrinsic signature for %s", v.Op, name)
	}
	return lfc.b.CreateCall(sig, fn, args, "")
}

// Use integer lane types for routing floating-point payloads as well. There is
// no arithmetic conversion, so NaNs and signed zeros keep their exact bits.
func (lfc *LLVMFuncContext) simdPermuteCall(v *Value, concat bool, x, y, indices llvm.Value, bits, width int) llvm.Value {
	name := ""
	if concat {
		name = fmt.Sprintf("llvm.x86.avx512.vpermi2var.%s.%d", map[int]string{8: "qi", 16: "hi", 32: "d", 64: "q"}[bits], width)
		return lfc.simdShuffleCall(v, name, x, indices, y)
	}
	if bits == 32 && width == 256 {
		name = "llvm.x86.avx2.permd"
	} else {
		name = fmt.Sprintf("llvm.x86.avx512.permvar.%s.%d", map[int]string{8: "qi", 16: "hi", 32: "si", 64: "di"}[bits], width)
	}
	return lfc.simdShuffleCall(v, name, x, indices)
}

func (lfc *LLVMFuncContext) simdDynamicShuffleTarget(v *Value, info goALLCSIMDOpInfo, x, y, indices llvm.Value, lanes int) llvm.Value {
	bits := int(info.laneBits)
	width := bits * lanes
	vector := indices.Type()
	x = lfc.b.CreateBitCast(x, vector, "")
	if !y.IsNil() {
		y = lfc.b.CreateBitCast(y, vector, "")
	}
	var result llvm.Value
	switch info.lowering {
	case goALLCSIMDLowerLookupOrZero:
		result = lfc.simdShuffleCall(v, "llvm.aarch64.neon.tbl1", x, indices)
	case goALLCSIMDLowerPermuteOrZero, goALLCSIMDLowerPermuteOrZero128:
		name := map[int]string{128: "llvm.x86.ssse3.pshuf.b.128", 256: "llvm.x86.avx2.pshuf.b", 512: "llvm.x86.avx512.pshuf.b.512"}[width]
		result = lfc.simdShuffleCall(v, name, x, indices)
	default:
		result = lfc.simdPermuteCall(v, info.lowering == goALLCSIMDLowerConcatPermute, x, y, indices, bits, width)
	}
	return lfc.simdLaneResult(v, result)
}
