// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "fmt"

func goALLCStaticShuffleLowering(lowering string) bool {
	switch lowering {
	case "get-low", "get-high", "set-low", "set-high", "broadcast-low",
		"interleave-low", "interleave-high", "interleave-low-128", "interleave-high-128",
		"concat-even", "concat-odd", "interleave-even", "interleave-odd":
		return true
	}
	return false
}

// Static shuffles retain lane type but can change vector width. Validate the
// public operand shapes instead of copying native register widths into the
// descriptor. All indices are derived from these shapes by the LLVM lowering.
func validateGoALLCStaticShuffle(op, genericOp Operation, lowering string) {
	base, bits, lanes, ok := goALLCLaneFromGoType(genericOp.In[0].Go)
	if !ok || lanes < 2 || lanes%2 != 0 || (op.OperandOrder != nil && *op.OperandOrder != "") {
		panic(fmt.Errorf("simdgen: LLVM static shuffle has invalid source shape/order for %s", op.GenericName()))
	}
	check := func(operand Operand, wantLanes int) {
		b, e, n, valid := goALLCLaneFromGoType(operand.Go)
		if !valid || b != base || e != bits || n != wantLanes || operand.Class != "vreg" ||
			operand.TreatLikeAScalarOfSize != nil || operand.Bits == nil || *operand.Bits != bits*n ||
			(bits*n != 128 && bits*n != 256 && bits*n != 512) {
			panic(fmt.Errorf("simdgen: LLVM static shuffle has incompatible lane shape for %s", op.GenericName()))
		}
	}
	check(genericOp.In[0], lanes)
	outLanes := lanes
	switch lowering {
	case "get-low", "get-high":
		outLanes = lanes / 2
	case "set-low", "set-high":
		check(genericOp.In[1], lanes/2)
	case "broadcast-low":
		_, _, outLanes, _ = goALLCLaneFromGoType(genericOp.Out[0].Go)
		if bits*lanes != 128 || outLanes < lanes {
			panic(fmt.Errorf("simdgen: LLVM broadcast has invalid width for %s", op.GenericName()))
		}
	default:
		check(genericOp.In[1], lanes)
		if (lowering == "interleave-low-128" || lowering == "interleave-high-128") && bits*lanes%128 != 0 {
			panic(fmt.Errorf("simdgen: LLVM grouped shuffle has invalid group width for %s", op.GenericName()))
		}
	}
	check(genericOp.Out[0], outLanes)
}
