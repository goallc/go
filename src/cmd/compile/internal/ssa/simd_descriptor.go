// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

type goALLCSIMDLowering uint8

const (
	goALLCSIMDLowerNone goALLCSIMDLowering = iota
	goALLCSIMDLowerAdd
	goALLCSIMDLowerSub
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
	goALLCSIMDLowerEqual
	goALLCSIMDLowerNotEqual
	goALLCSIMDLowerGreater
	goALLCSIMDLowerGreaterEqual
	goALLCSIMDLowerLess
	goALLCSIMDLowerLessEqual
)

type goALLCSIMDArch uint8

const (
	goALLCSIMDArchNone  goALLCSIMDArch = 0
	goALLCSIMDArchAmd64 goALLCSIMDArch = 1 << (iota - 1)
	goALLCSIMDArchArm64
	goALLCSIMDArchWasm
)

type goALLCSIMDLane uint8

const (
	goALLCSIMDLaneNone goALLCSIMDLane = iota
	goALLCSIMDLaneInt
	goALLCSIMDLaneUint
	goALLCSIMDLaneFloat
)

type goALLCSIMDInput uint8

const (
	goALLCSIMDInputInvalid goALLCSIMDInput = iota
	goALLCSIMDInputPureVreg
	goALLCSIMDInputVregMask
	goALLCSIMDInputVregImmediate
	goALLCSIMDInputVregMaskImmediate
	goALLCSIMDInputPureMask
	goALLCSIMDInputVregList
)

type goALLCSIMDOutput uint8

const (
	goALLCSIMDOutputInvalid goALLCSIMDOutput = iota
	goALLCSIMDOutputNone
	goALLCSIMDOutputVreg
	goALLCSIMDOutputGreg
	goALLCSIMDOutputMask
	goALLCSIMDOutputVregAtInput
	goALLCSIMDOutputVregScalar
)

type goALLCSIMDImmediate uint8

const (
	goALLCSIMDImmediateInvalid goALLCSIMDImmediate = iota
	goALLCSIMDImmediateNone
	goALLCSIMDImmediateConst
	goALLCSIMDImmediateVariable
	goALLCSIMDImmediateConstVariable
	goALLCSIMDImmediateVariableLimited
)

type goALLCSIMDMask uint8

const (
	goALLCSIMDMaskInvalid goALLCSIMDMask = iota
	goALLCSIMDMaskNone
	goALLCSIMDMaskOne
	goALLCSIMDMaskAll
)

// goALLCSIMDArchInfo preserves the architecture-specific source facts used by
// simdgen. The lowering only consumes operandOrder and cpuProfile today; the
// remaining fields make generator drift and future feature/memory lowering
// explicit instead of requiring the compiler to rediscover it from op names.
type goALLCSIMDArchInfo struct {
	cpuFeature        string
	cpuProfile        string
	operandOrder      string
	input             string
	output            string
	immediate         string
	mask              string
	inputs            string
	outputs           string
	memoryFeature     string
	memoryFeatureData string
}

type goALLCSIMDOpInfo struct {
	lowering  goALLCSIMDLowering
	archs     goALLCSIMDArch
	width     uint16
	lane      goALLCSIMDLane
	laneBits  uint8
	lanes     uint8
	input     goALLCSIMDInput
	output    goALLCSIMDOutput
	immediate goALLCSIMDImmediate
	mask      goALLCSIMDMask
	memory    string
	inputs    string
	outputs   string
	amd64     goALLCSIMDArchInfo
	arm64     goALLCSIMDArchInfo
	wasm      goALLCSIMDArchInfo
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
	case "wasm":
		return info.wasm
	default:
		return goALLCSIMDArchInfo{}
	}
}
