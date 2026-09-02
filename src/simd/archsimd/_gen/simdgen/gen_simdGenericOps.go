// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"simd/archsimd/_gen/sgutil"
)

// writeSIMDGenericOps generates the generic ops for the current architecture,
// merges them with existing ops from other architectures, and returns the
// result as a buffer ready for writing.
func writeSIMDGenericOps(ops []Operation, genericOpsFilePath string) *bytes.Buffer {
	// Generate fresh ops for current arch.
	currentArch := CurrentArch().Arch
	var newOps []sgutil.GenericOpsData
	for _, op := range ops {
		if op.NoGenericOps != nil && *op.NoGenericOps == "true" {
			continue
		}
		if op.SkipMaskedMethod() {
			continue
		}
		_, _, _, immType, gOp, _ := op.shape()
		genericIn, genericOut, genericMask, _, _, _ := gOp.shape()
		// Generic SSA output representation follows the Go result type, not
		// an architecture's destructive output or mask/scalar register bank.
		genericOut = goALLCGenericOutputShape(gOp, genericOut)
		genericImm := immType
		switch immType {
		case ConstImm:
			// A constant instruction predicate is part of the named generic
			// operation's semantics rather than an SSA Aux value.
			genericImm = NoImm
		case VarImm, VarImmLim, ConstVarImm:
			// Generic SSA carries each user-visible immediate in the same
			// UInt8 Aux slot. Instruction offsets and limits remain in the
			// architecture-specific descriptor.
			genericImm = VarImm
		}

		genericName := gOp.GenericName()
		simd := goALLCSIMDDescriptor(op, gOp, genericIn, genericOut, genericMask, genericImm)
		if simd.IsZero() {
			if _, ok := goALLCSIMDPlanForGenericOp(genericName); !ok {
				panic(fmt.Errorf("simdgen: generic op %q has neither an LLVM lowering nor a reviewed GoALLC plan", genericName))
			}
		}

		newOps = append(newOps, sgutil.GenericOpsData{
			OpName:  genericName,
			OpInLen: len(gOp.In),
			Comm:    op.Commutative,
			HasAux:  immType == VarImm || immType == VarImmLim || immType == ConstVarImm,
			Archs:   []string{currentArch},
			SIMD:    simd,
		})
	}

	buf := sgutil.MergeSIMDGenericOps(newOps, genericOpsFilePath, currentArch)

	return buf
}
