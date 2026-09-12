// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "fmt"

func goALLCDynamicShuffleLowering(lowering string) bool {
	switch lowering {
	case "permute", "concat-permute", "lookup-or-zero", "permute-or-zero", "permute-or-zero-128":
		return true
	}
	return false
}

// Permute and ConcatPermute keep indices first in generic SSA; the other
// families keep the data first. Preserve those existing frontend contracts.
// Index lanes have the data lane width even for floating-point data. Reject
// mismatches here rather than guessing a bitcast or index interpretation later.
func validateGoALLCDynamicShuffle(op, genericOp Operation, lowering string) {
	data, index := 0, len(genericOp.In)-1
	if lowering == "permute" || lowering == "concat-permute" {
		data, index = 1, 0
	}
	base, bits, lanes, ok := goALLCLaneFromGoType(genericOp.In[data].Go)
	valid := ok && (base == "int" || base == "uint" || base == "float") &&
		(bits*lanes == 128 || bits*lanes == 256 || bits*lanes == 512) &&
		(base != "float" || bits == 32 || bits == 64)
	order, wantOrder, indexBase := "", "", "uint"
	if op.OperandOrder != nil {
		order = *op.OperandOrder
	}
	switch lowering {
	case "permute":
		wantOrder = "21Type1"
		valid = valid && (bits < 32 || bits*lanes > 128)
	case "concat-permute":
		wantOrder = "231Type1"
	case "lookup-or-zero":
		valid = valid && bits == 8 && lanes == 16 && base != "float"
		indexBase = base
	case "permute-or-zero", "permute-or-zero-128":
		valid = valid && bits == 8 && base != "float"
		if lowering == "permute-or-zero" {
			valid = valid && lanes == 16
		} else {
			valid = valid && lanes > 16
		}
		indexBase = "int"
	default:
		valid = false
	}
	if !valid || order != wantOrder {
		panic(fmt.Errorf("simdgen: LLVM dynamic shuffle has invalid shape/order for %s", op.GenericName()))
	}
	for i, operand := range append(append([]Operand(nil), genericOp.In...), genericOp.Out...) {
		// ARM64's one-register table is tagged as list member zero. A
		// multi-register table requires a separate, wider source contract.
		if operand.ListNumber != nil && (lowering != "lookup-or-zero" || i != 0 || *operand.ListNumber != 0) {
			panic(fmt.Errorf("simdgen: LLVM dynamic shuffle has invalid register list for %s", op.GenericName()))
		}
		wantBase := base
		if i == index {
			wantBase = indexBase
		}
		b, e, n, valid := goALLCLaneFromGoType(operand.Go)
		if !valid || b != wantBase || e != bits || n != lanes || operand.Class != "vreg" ||
			operand.TreatLikeAScalarOfSize != nil || operand.Bits == nil || *operand.Bits != bits*lanes {
			panic(fmt.Errorf("simdgen: LLVM dynamic shuffle has incompatible lanes for %s", op.GenericName()))
		}
	}
}
