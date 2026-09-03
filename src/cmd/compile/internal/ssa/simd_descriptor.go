// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

type goALLCSIMDLowering uint8

const (
	goALLCSIMDLowerNone goALLCSIMDLowering = iota
	goALLCSIMDLowerAdd
	goALLCSIMDLowerSub
	goALLCSIMDLowerAddSaturated
	goALLCSIMDLowerSubSaturated
	goALLCSIMDLowerExtractElement
	goALLCSIMDLowerInsertElement
	goALLCSIMDLowerReduceAdd
	goALLCSIMDLowerReduceMax
	goALLCSIMDLowerReduceMin
	goALLCSIMDLowerMul
	goALLCSIMDLowerDiv
	goALLCSIMDLowerAnd
	goALLCSIMDLowerOr
	goALLCSIMDLowerXor
	goALLCSIMDLowerAndNot
	goALLCSIMDLowerOrNot
	goALLCSIMDLowerNot
	goALLCSIMDLowerNeg
	goALLCSIMDLowerAbs
	goALLCSIMDLowerSqrt
	goALLCSIMDLowerRoundEven
	goALLCSIMDLowerFloor
	goALLCSIMDLowerCeil
	goALLCSIMDLowerTrunc
	goALLCSIMDLowerOnesCount
	goALLCSIMDLowerLeadingZeros
	goALLCSIMDLowerMax
	goALLCSIMDLowerMin
	goALLCSIMDLowerEqual
	goALLCSIMDLowerNotEqual
	goALLCSIMDLowerGreater
	goALLCSIMDLowerGreaterEqual
	goALLCSIMDLowerLess
	goALLCSIMDLowerLessEqual
)

type goALLCSIMDLane uint8

const (
	goALLCSIMDLaneInvalid goALLCSIMDLane = iota
	goALLCSIMDLaneInt
	goALLCSIMDLaneUint
	goALLCSIMDLaneFloat
)

// goALLCSIMDArchInfo contains only architecture-specific lowering decisions.
// Source instruction shapes are validated by simdgen instead of being copied
// into the compiler's generated tables.
type goALLCSIMDArchInfo struct {
	cpuProfile   string
	operandOrder string
}

type goALLCSIMDOpInfo struct {
	lowering goALLCSIMDLowering
	lane     goALLCSIMDLane
	laneBits uint8
	amd64    goALLCSIMDArchInfo
	arm64    goALLCSIMDArchInfo
}

func goALLCSIMDInfo(op Op) (goALLCSIMDOpInfo, bool) {
	if op < 0 || int(op) >= len(goALLCSIMDOpcodeIndex) {
		return goALLCSIMDOpInfo{}, false
	}
	index := goALLCSIMDOpcodeIndex[op]
	if index == 0 {
		return goALLCSIMDOpInfo{}, false
	}
	return goALLCSIMDOpTable[index], true
}

func (info goALLCSIMDOpInfo) archInfo(arch string) goALLCSIMDArchInfo {
	switch arch {
	case "amd64":
		return info.amd64
	case "arm64":
		return info.arm64
	default:
		return goALLCSIMDArchInfo{}
	}
}
