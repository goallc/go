// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "fmt"

func goALLCImmediateShuffleLowering(lowering string) bool {
	switch lowering {
	case "permute-32-128", "permute-low-16-128", "permute-high-16-128",
		"concat-select-128", "concat-permute-128", "concat-shift-bytes-128":
		return true
	}
	return false
}

func validateGoALLCImmediateShuffle(op, genericOp Operation, lowering string) {
	base, bits, lanes, ok := goALLCLaneFromGoType(genericOp.In[0].Go)
	order, wantOrder := "", ""
	if op.OperandOrder != nil {
		order = *op.OperandOrder
	}
	valid := ok && (bits*lanes == 128 || bits*lanes == 256 || bits*lanes == 512)
	switch lowering {
	case "permute-32-128":
		valid = valid && bits == 32 && base != "float"
	case "permute-low-16-128", "permute-high-16-128":
		valid = valid && bits == 16 && base != "float"
	case "concat-select-128":
		valid = valid && (bits == 32 || bits == 64)
	case "concat-permute-128":
		valid = valid && bits*lanes == 256
		wantOrder = "II"
	case "concat-shift-bytes-128":
		valid = valid && bits == 8 && base == "uint"
		wantOrder = "2I"
	default:
		valid = false
	}
	if !valid || order != wantOrder {
		panic(fmt.Errorf("simdgen: LLVM immediate shuffle has invalid shape/order for %s", op.GenericName()))
	}
	for _, operands := range [][]Operand{genericOp.In, genericOp.Out} {
		for _, operand := range operands {
			b, e, n, valid := goALLCLaneFromGoType(operand.Go)
			if !valid || b != base || e != bits || n != lanes || operand.Class != "vreg" ||
				operand.TreatLikeAScalarOfSize != nil || operand.Bits == nil || *operand.Bits != bits*lanes {
				panic(fmt.Errorf("simdgen: LLVM immediate shuffle has incompatible lanes for %s", op.GenericName()))
			}
		}
	}
}
