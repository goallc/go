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

func TestLLVMGeneratedSIMDWidenMultiply(t *testing.T) {
	oldTypes, oldModule := type2lTypes, CurrentModule
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() { type2lTypes, CurrentModule = oldTypes, oldModule }()
	for _, tc := range []struct {
		op           Op
		bits, lanes  int
		signed, even bool
	}{
		{OpMulWidenEvenInt32x4, 32, 4, true, true},
		{OpMulWidenEvenInt32x8, 32, 8, true, true},
		{OpMulWidenEvenUint32x4, 32, 4, false, true},
		{OpMulWidenEvenUint32x8, 32, 8, false, true},
		{OpMulWidenLoInt8x16, 8, 16, true, false},
		{OpMulWidenLoInt16x8, 16, 8, true, false},
		{OpMulWidenLoInt32x4, 32, 4, true, false},
		{OpMulWidenLoUint8x16, 8, 16, false, false},
		{OpMulWidenLoUint16x8, 16, 8, false, false},
		{OpMulWidenLoUint32x4, 32, 4, false, false},
	} {
		t.Run(tc.op.String(), func(t *testing.T) {
			kinds := map[int]types.Kind{8: types.TUINT8, 16: types.TUINT16, 32: types.TUINT32, 64: types.TUINT64}
			if tc.signed {
				kinds = map[int]types.Kind{8: types.TINT8, 16: types.TINT16, 32: types.TINT32, 64: types.TINT64}
			}
			input := llvmTestSIMDType("widen-input", types.Types[kinds[tc.bits]], int64(tc.lanes))
			output := llvmTestSIMDType("widen-output", types.Types[kinds[2*tc.bits]], int64(tc.lanes/2))
			module := GlobalCtxt.NewModule("widen-multiply")
			defer module.Dispose()
			CurrentModule = module
			builder := GlobalCtxt.NewBuilder()
			defer builder.Dispose()
			fn := llvm.AddFunction(module, "multiply", llvm.FunctionType(getLLVMType(output), nil, false))
			builder.SetInsertPointAtEnd(llvm.AddBasicBlock(fn, "entry"))
			xArg, yArg := &Value{ID: 1, Op: OpArg, Type: input}, &Value{ID: 2, Op: OpArg, Type: input}
			v := &Value{ID: 3, Op: tc.op, Type: output, Args: []*Value{xArg, yArg}}
			arch := "arm64"
			if tc.even {
				arch = "amd64"
			}
			info, _ := goALLCSIMDInfo(tc.op)
			mask := ^uint64(0) >> (64 - tc.bits)
			sign := uint64(1) << (tc.bits - 1)
			data := []uint64{0, 1, mask, sign, sign - 1, sign + 1, mask - 1, 0xa55aa55a & mask}
			var result llvm.Value
			// Cross edge values, rotating them through distinct lanes to catch
			// wrong selection as well as multiplying before widening.
			for a := range data {
				for b := range data {
					xs, ys := make([]llvm.Value, tc.lanes), make([]llvm.Value, tc.lanes)
					for i := range xs {
						xs[i] = llvm.ConstInt(GlobalCtxt.IntType(tc.bits), data[(a+i)%len(data)], false)
						ys[i] = llvm.ConstInt(GlobalCtxt.IntType(tc.bits), data[(b+3*i)%len(data)], false)
					}
					ctx := &LLVMFuncContext{
						F:           &Func{Config: &Config{arch: arch}, Entry: &Block{CPUfeatures: CPUavx | CPUavx2 | CPUavx512}},
						CPUFeatures: &llvmCPUFeaturePlan{floor: info.archInfo(arch).cpuProfile},
						Vs:          map[ID]llvm.Value{1: llvm.ConstVector(xs, false), 2: llvm.ConstVector(ys, false)}, b: builder,
					}
					var ok bool
					result, ok = ctx.lowerGeneratedSIMD(v)
					if !ok {
						t.Fatal("widening multiply is not lowered")
					}
					if result.Type() != getLLVMType(output) {
						t.Fatalf("wrong result type: %s", result.String())
					}
					for i := 0; i < tc.lanes/2; i++ {
						j := i
						if tc.even {
							j *= 2
						}
						x, y := data[(a+j)%len(data)], data[(b+3*j)%len(data)]
						want := x * y
						if tc.signed {
							sx, sy := int64(x<<(64-tc.bits))>>(64-tc.bits), int64(y<<(64-tc.bits))>>(64-tc.bits)
							want = uint64(sx * sy)
						}
						want &= ^uint64(0) >> (64 - 2*tc.bits)
						got := llvm.ConstExtractElement(result, llvm.ConstInt(GlobalCtxt.Int32Type(), uint64(i), false))
						if got.IsAConstantInt().IsNil() || got.ZExtValue() != want {
							t.Fatalf("x=%#x y=%#x lane=%d got=%s want=%#x", x, y, i, got.String(), want)
						}
					}
				}
			}
			builder.CreateRet(result)
			if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
				t.Fatal(err)
			}
		})
	}
}
