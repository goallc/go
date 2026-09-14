// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/types"
	"testing"

	"github.com/goallc/go-llvm"
)

// Exercise private masked operations through GenLV, including noncanonical
// masks: only each lane's sign bit is significant, not any other set bit.
func TestLLVMGeneratedSIMDMaskSelect(t *testing.T) {
	oldTypes, oldModule := type2lTypes, CurrentModule
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() { type2lTypes, CurrentModule = oldTypes, oldModule }()
	for _, test := range []struct {
		op                 Op
		width, resultWidth int
	}{
		{OpblendMaskedInt8x64, 512, 512},
		{OpblendMaskedInt16x32, 512, 512},
		{OpblendMaskedInt32x16, 512, 512},
		{OpblendMaskedInt64x8, 512, 512},
		{Opbroadcast1To16MaskedInt8x16, 128, 128},
		{Opbroadcast1To32MaskedInt8x16, 128, 256},
		{Opbroadcast1To64MaskedInt8x16, 128, 512},
		{Opbroadcast1To16MaskedUint8x16, 128, 128},
		{Opbroadcast1To32MaskedUint8x16, 128, 256},
		{Opbroadcast1To64MaskedUint8x16, 128, 512},
		{Opbroadcast1To8MaskedInt16x8, 128, 128},
		{Opbroadcast1To16MaskedInt16x8, 128, 256},
		{Opbroadcast1To32MaskedInt16x8, 128, 512},
		{Opbroadcast1To8MaskedUint16x8, 128, 128},
		{Opbroadcast1To16MaskedUint16x8, 128, 256},
		{Opbroadcast1To32MaskedUint16x8, 128, 512},
		{Opbroadcast1To4MaskedFloat32x4, 128, 128},
		{Opbroadcast1To8MaskedFloat32x4, 128, 256},
		{Opbroadcast1To16MaskedFloat32x4, 128, 512},
		{Opbroadcast1To4MaskedInt32x4, 128, 128},
		{Opbroadcast1To8MaskedInt32x4, 128, 256},
		{Opbroadcast1To16MaskedInt32x4, 128, 512},
		{Opbroadcast1To4MaskedUint32x4, 128, 128},
		{Opbroadcast1To8MaskedUint32x4, 128, 256},
		{Opbroadcast1To16MaskedUint32x4, 128, 512},
		{Opbroadcast1To2MaskedFloat64x2, 128, 128},
		{Opbroadcast1To4MaskedFloat64x2, 128, 256},
		{Opbroadcast1To8MaskedFloat64x2, 128, 512},
		{Opbroadcast1To2MaskedInt64x2, 128, 128},
		{Opbroadcast1To4MaskedInt64x2, 128, 256},
		{Opbroadcast1To8MaskedInt64x2, 128, 512},
		{Opbroadcast1To2MaskedUint64x2, 128, 128},
		{Opbroadcast1To4MaskedUint64x2, 128, 256},
		{Opbroadcast1To8MaskedUint64x2, 128, 512},
	} {
		t.Run(test.op.String(), func(t *testing.T) {
			info, _ := goALLCSIMDInfo(test.op)
			bits := int(info.laneBits)
			lanes, resultLanes := test.width/bits, test.resultWidth/bits
			carriers := map[int]*types.Type{128: types.TypeVec128, 256: types.TypeVec256, 512: types.TypeVec512}
			module := GlobalCtxt.NewModule("mask")
			CurrentModule = module
			builder := GlobalCtxt.NewBuilder()
			defer module.Dispose()
			defer builder.Dispose()
			fn := llvm.AddFunction(module, "mask", llvm.FunctionType(getLLVMType(carriers[test.resultWidth]), nil, false))
			builder.SetInsertPointAtEnd(llvm.AddBasicBlock(fn, "entry"))
			args := []*Value{{ID: 1, Op: OpArg, Type: carriers[test.width]}, {ID: 2, Op: OpArg, Type: carriers[test.width]}, {ID: 3, Op: OpArg, Type: carriers[test.width]}}
			laneType := GlobalCtxt.IntType(bits)
			mapped := map[ID]llvm.Value{}
			values := [2][]llvm.Value{make([]llvm.Value, lanes), make([]llvm.Value, lanes)}
			for j := range values {
				for i := range lanes {
					values[j][i] = llvm.ConstInt(laneType, uint64(17+i+80*j), false)
				}
				mapped[ID(j+1)] = llvm.ConstBitCast(llvm.ConstVector(values[j], false), getLLVMType(carriers[test.width]))
			}
			var result llvm.Value
			for pattern := range 256 {
				mask := make([]llvm.Value, lanes)
				for i := range mask {
					// A rotating byte exercises zero, positive, negative, and all-ones
					// masks without assuming canonical comparison results.
					m := uint64(uint8(pattern+i)) << (bits - 8)
					mask[i] = llvm.ConstInt(laneType, m, false)
				}
				mapped[3] = llvm.ConstBitCast(llvm.ConstVector(mask, false), getLLVMType(carriers[test.width]))
				operands := args
				if info.lowering == goALLCSIMDLowerBroadcastLowMasked {
					operands = []*Value{args[0], args[2]}
				}
				ctx := &LLVMFuncContext{
					F:           &Func{Config: &Config{arch: "amd64"}, Entry: &Block{CPUfeatures: CPUavx512}},
					CPUFeatures: &llvmCPUFeaturePlan{floor: info.archInfo("amd64").cpuProfile},
					Vs:          mapped, b: builder,
				}
				v := &Value{ID: 4, Op: test.op, Type: carriers[test.resultWidth], Args: operands}
				delete(mapped, v.ID)
				result = ctx.GenLV(v)
				integerResult := llvm.ConstBitCast(result, llvm.VectorType(laneType, resultLanes))
				for i := range resultLanes {
					selected := i < lanes && uint8(pattern+i)&128 != 0
					want := uint64(0)
					if info.lowering == goALLCSIMDLowerBlendMasked {
						want = uint64(17 + i)
						if selected {
							want += 80
						}
					} else if selected {
						want = 17
					}
					got := llvm.ConstExtractElement(integerResult, llvm.ConstInt(GlobalCtxt.Int32Type(), uint64(i), false))
					if got.IsAConstantInt().IsNil() || got.ZExtValue() != want {
						t.Fatalf("pattern=%#x lane=%d got=%s want=%#x", pattern, i, got.String(), want)
					}
				}
			}
			builder.CreateRet(llvm.ConstBitCast(result, getLLVMType(carriers[test.resultWidth])))
			if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
				t.Fatal(err)
			}
		})
	}
}
