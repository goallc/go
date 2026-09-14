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
	goALLCSIMDLowerScale
	goALLCSIMDLowerReciprocal
	goALLCSIMDLowerReciprocalSqrt
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
	goALLCSIMDLowerAverage
	goALLCSIMDLowerLeadingSignBits
	goALLCSIMDLowerMulHigh
	goALLCSIMDLowerMulWidenEven
	goALLCSIMDLowerMulWidenLow
	goALLCSIMDLowerCarrylessMulWidenLow
	goALLCSIMDLowerCarrylessMul
	goALLCSIMDLowerAESEncrypt
	goALLCSIMDLowerAESEncryptLast
	goALLCSIMDLowerAESDecrypt
	goALLCSIMDLowerAESDecryptLast
	goALLCSIMDLowerAESKeygen
	goALLCSIMDLowerAESInverseMix
	goALLCSIMDLowerGFMul
	goALLCSIMDLowerGFAffine
	goALLCSIMDLowerGFAffineInverse
	goALLCSIMDLowerSHA1Rounds
	goALLCSIMDLowerSHA1NextE
	goALLCSIMDLowerSHA1Message1
	goALLCSIMDLowerSHA1Message2
	goALLCSIMDLowerSHA256Rounds
	goALLCSIMDLowerSHA256Message1
	goALLCSIMDLowerSHA256Message2
	goALLCSIMDLowerMulSign
	goALLCSIMDLowerMulAddInteger
	goALLCSIMDLowerFMA
	goALLCSIMDLowerFMAAddEven
	goALLCSIMDLowerFMAAddOdd
	goALLCSIMDLowerPairAdd
	goALLCSIMDLowerDotPairs
	goALLCSIMDLowerRotateLeft
	goALLCSIMDLowerRotateRight
	goALLCSIMDLowerFunnelLeft
	goALLCSIMDLowerFunnelRight
	goALLCSIMDLowerFunnelAllLeft
	goALLCSIMDLowerFunnelAllRight
	goALLCSIMDLowerDotPairsUSSat
	goALLCSIMDLowerSum8AbsDiff
	goALLCSIMDLowerPairSub
	goALLCSIMDLowerPairAdd128
	goALLCSIMDLowerPairSub128
	goALLCSIMDLowerPairSAddSat
	goALLCSIMDLowerPairSSubSat
	goALLCSIMDLowerPairSAddSat128
	goALLCSIMDLowerPairSSubSat128
	goALLCSIMDLowerAddOddSubEven
	goALLCSIMDLowerExtendInteger
	goALLCSIMDLowerTruncateInteger
	goALLCSIMDLowerSaturateInteger
	goALLCSIMDLowerSaturateIntegerPack128
	goALLCSIMDLowerConvertFloat
	goALLCSIMDLowerGetLow
	goALLCSIMDLowerGetHigh
	goALLCSIMDLowerSetLow
	goALLCSIMDLowerSetHigh
	goALLCSIMDLowerBroadcastLow
	goALLCSIMDLowerInterleaveLow
	goALLCSIMDLowerInterleaveHigh
	goALLCSIMDLowerInterleaveLow128
	goALLCSIMDLowerInterleaveHigh128
	goALLCSIMDLowerConcatEven
	goALLCSIMDLowerConcatOdd
	goALLCSIMDLowerInterleaveEven
	goALLCSIMDLowerInterleaveOdd
	goALLCSIMDLowerPermute
	goALLCSIMDLowerConcatPermute
	goALLCSIMDLowerLookupOrZero
	goALLCSIMDLowerPermuteOrZero
	goALLCSIMDLowerPermuteOrZero128
	goALLCSIMDLowerPermute32_128
	goALLCSIMDLowerPermuteLow16_128
	goALLCSIMDLowerPermuteHigh16_128
	goALLCSIMDLowerConcatSelect128
	goALLCSIMDLowerConcatPermute128
	goALLCSIMDLowerConcatShiftBytes128
	goALLCSIMDLowerShiftAllLeft
	goALLCSIMDLowerShiftAllRight
	goALLCSIMDLowerShiftLeft
	goALLCSIMDLowerShiftRight
	goALLCSIMDLowerShiftSignedCount
	goALLCSIMDLowerShiftSignedCountSaturated
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
	lowering       goALLCSIMDLowering
	lane           goALLCSIMDLane
	laneBits       uint8
	resultLaneBits uint8          // conversions only; TypeVec retains width, not lane shape
	resultLane     goALLCSIMDLane // invalid means the same kind as lane
	amd64          goALLCSIMDArchInfo
	arm64          goALLCSIMDArchInfo
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
