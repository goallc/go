// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "fmt"

func goALLCShiftLowering(lowering string) bool {
	switch lowering {
	case "shift-all-left", "shift-all-right", "shift-left", "shift-right",
		"shift-signed-count", "shift-signed-saturated":
		return true
	}
	return false
}

// Ordinary shifts keep data first and an unsigned count second in generic
// SSA. Scalar counts retain all 64 bits; vector counts have the same lane
// width and lane count as the data. Neither form masks oversized counts.
// ARM64 signed-count shifts instead use signed vector lanes.
func validateGoALLCShift(op, genericOp Operation, lowering string) {
	if op.Commutative || genericOp.Commutative ||
		(op.OperandOrder != nil && *op.OperandOrder != "") ||
		(genericOp.OperandOrder != nil && *genericOp.OperandOrder != "") {
		panic(fmt.Errorf("simdgen: LLVM shift has invalid operand order for %s", op.GenericName()))
	}
	for _, operand := range append(append([]Operand(nil), genericOp.In...), genericOp.Out...) {
		if operand.ListNumber != nil || operand.Const != nil || operand.ImmOffset != nil || operand.ImmMax != nil {
			panic(fmt.Errorf("simdgen: LLVM shift has invalid list/immediate operand for %s", op.GenericName()))
		}
	}
	base, bits, lanes, ok := goALLCLaneFromGoType(genericOp.In[0].Go)
	if !ok || (base != "int" && base != "uint") ||
		(bits*lanes != 128 && bits*lanes != 256 && bits*lanes != 512) {
		panic(fmt.Errorf("simdgen: LLVM shift has invalid integer data shape for %s", op.GenericName()))
	}
	checkVector := func(operand Operand, wantBase string) {
		b, e, n, valid := goALLCLaneFromGoType(operand.Go)
		if !valid || b != wantBase || e != bits || n != lanes || operand.Class != "vreg" ||
			operand.TreatLikeAScalarOfSize != nil || operand.Bits == nil || *operand.Bits != bits*lanes {
			panic(fmt.Errorf("simdgen: LLVM shift has incompatible vector lanes for %s", op.GenericName()))
		}
	}
	checkVector(genericOp.In[0], base)
	checkVector(genericOp.Out[0], base)
	count := genericOp.In[1]
	if lowering == "shift-signed-count" || lowering == "shift-signed-saturated" {
		checkVector(count, "int")
		return
	}
	if lowering == "shift-left" || lowering == "shift-right" {
		checkVector(count, "uint")
		return
	}
	if count.TreatLikeAScalarOfSize != nil {
		// op2VecAsScalar declares uint{TreatLikeAScalarOfSize}, regardless
		// of the native vector's signedness or lane size. In particular,
		// ARM64's VSSHL/VUSHL metadata uses signed native count lanes.
		b, e, n, valid := goALLCLaneFromGoType(count.Go)
		if *count.TreatLikeAScalarOfSize != 64 || count.Class != "vreg" ||
			!valid || (b != "int" && b != "uint") || count.Bits == nil || *count.Bits != e*n ||
			(e*n != 128 && e*n != 256 && e*n != 512) {
			panic(fmt.Errorf("simdgen: LLVM shift requires a uint64 scalar count for %s", op.GenericName()))
		}
		return
	}
	if count.Class != "greg" || count.Go == nil || *count.Go != "uint64" ||
		count.Bits == nil || *count.Bits != 64 ||
		(count.ElemBits != nil && *count.ElemBits != 64) || (count.Lanes != nil && *count.Lanes != 1) {
		panic(fmt.Errorf("simdgen: LLVM shift requires a uint64 scalar count for %s", op.GenericName()))
	}
}
