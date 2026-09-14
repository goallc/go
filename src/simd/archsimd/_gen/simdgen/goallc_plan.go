// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"regexp"
)

// goALLCSIMDPlan groups generic SIMD operations by the LLVM lowering
// mechanism that will implement them. It deliberately classifies semantic
// operation families rather than generated type and width instances.
//
// Implemented operations carry an LLVMLowering descriptor and do not appear
// here. Keeping the pending plan separate also avoids copying planning data
// into cmd/compile's generated opcode table.
type goALLCSIMDPlan uint8

const (
	goALLCSIMDPlanInvalid goALLCSIMDPlan = iota
	goALLCSIMDPlanStandard
	goALLCSIMDPlanCompose
	goALLCSIMDPlanConvert
	goALLCSIMDPlanShuffle
	goALLCSIMDPlanMask
	goALLCSIMDPlanShift
	goALLCSIMDPlanTargetIntrinsic
	goALLCSIMDPlanLegacy
)

func (p goALLCSIMDPlan) String() string {
	switch p {
	case goALLCSIMDPlanStandard:
		return "standard"
	case goALLCSIMDPlanCompose:
		return "compose"
	case goALLCSIMDPlanConvert:
		return "convert"
	case goALLCSIMDPlanShuffle:
		return "shuffle"
	case goALLCSIMDPlanMask:
		return "mask"
	case goALLCSIMDPlanShift:
		return "shift"
	case goALLCSIMDPlanTargetIntrinsic:
		return "target-intrinsic"
	case goALLCSIMDPlanLegacy:
		return "legacy"
	default:
		return "invalid"
	}
}

// goALLCSIMDPlannedFamilies is the reviewed coverage plan for generic SIMD
// operation families that do not yet have generated LLVM lowering metadata.
// A new semantic family must be classified here before simdgen will accept it.
// New lane types or vector widths of an existing family inherit its plan.
var goALLCSIMDPlannedFamilies = map[goALLCSIMDPlan][]string{
	// Operations expressible by one standard LLVM instruction or generic
	// intrinsic, subject to the semantic checks in their eventual lowering.
	goALLCSIMDPlanStandard: {},

	// Operations built from multiple target-independent LLVM instructions or
	// generic intrinsics.
	goALLCSIMDPlanCompose: {
		"blend", "tern",
	},

	// Lane conversion families now carry generated lowering descriptors.
	goALLCSIMDPlanConvert: {},

	// Pure data rearrangement. Constant and dynamic forms share this plan but
	// may select different standard shufflevector or IR expansion recipes.
	goALLCSIMDPlanShuffle: {},

	// Operations whose inactive-lane behavior or compacted memory/register
	// shape must be modeled explicitly.
	goALLCSIMDPlanMask: {
		"Compress", "Expand", "blendMasked",
		"broadcast1To2Masked", "broadcast1To4Masked",
		"broadcast1To8Masked", "broadcast1To16Masked",
		"broadcast1To32Masked", "broadcast1To64Masked",
	},

	// Shift and rotate families now carry generated lowering descriptors.
	goALLCSIMDPlanShift: {},

	// Operations whose exact semantics or useful implementation are tied to
	// existing target intrinsics. This does not authorize a Go-specific LLVM
	// intrinsic or a new target node.
	goALLCSIMDPlanTargetIntrinsic: {
		"CeilScaled", "CeilScaledResidue", "FloorScaled",
		"FloorScaledResidue", "GaloisFieldAffineTransform",
		"GaloisFieldAffineTransformInverse", "GaloisFieldMul", "Reciprocal",
		"ReciprocalSqrt", "RoundScaled", "RoundScaledResidue", "SHA1FourRounds",
		"SHA1Message1", "SHA1Message2", "SHA1NextE", "SHA256Message1",
		"SHA256Message2", "SHA256TwoRounds", "Scale", "TruncScaled",
		"TruncScaledResidue",
	},
}

// These generic operations predate the generated descriptor path and are
// still lowered by explicit cases in ssa2llvm.go. Keeping the exceptions exact
// prevents a wider operation with the same method stem from being silently
// treated as implemented.
var goALLCSIMDLegacyOps = map[string]struct{}{
	"bitSelectInt8x16":    {},
	"bitSelectNotInt8x16": {},
	"blendInt8x16":        {},
}

var goALLCSIMDTypeSuffixRE = regexp.MustCompile(`(?:Float|Int|Uint)[0-9]+x[0-9]+(?:x[0-9]+)?$`)

var goALLCSIMDPlanByFamily = func() map[string]goALLCSIMDPlan {
	result := make(map[string]goALLCSIMDPlan)
	for plan, families := range goALLCSIMDPlannedFamilies {
		if plan == goALLCSIMDPlanInvalid || plan == goALLCSIMDPlanLegacy {
			panic(fmt.Sprintf("invalid pending GoALLC SIMD plan %s", plan))
		}
		for _, family := range families {
			if old, ok := result[family]; ok {
				panic(fmt.Sprintf("GoALLC SIMD family %q has both %s and %s plans", family, old, plan))
			}
			result[family] = plan
		}
	}
	return result
}()

// goALLCSIMDPlanForGenericOp returns the reviewed plan for a generic operation
// that has no generated LLVM lowering descriptor.
func goALLCSIMDPlanForGenericOp(name string) (goALLCSIMDPlan, bool) {
	if _, ok := goALLCSIMDLegacyOps[name]; ok {
		return goALLCSIMDPlanLegacy, true
	}
	family := goALLCSIMDTypeSuffixRE.ReplaceAllString(name, "")
	plan, ok := goALLCSIMDPlanByFamily[family]
	return plan, ok
}
