// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/types"
	"math/bits"
	"testing"

	"github.com/goallc/go-llvm"
)

// Check private logic intrinsics through the production SSA lowering,
// against per-bit truth-table lookup and the native mask selection rules.
func TestLLVMGeneratedSIMDLogic(t *testing.T) {
	oldTypes, oldModule := type2lTypes, CurrentModule
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() { type2lTypes, CurrentModule = oldTypes, oldModule }()
	for _, op := range []Op{
		OpternInt32x4, OpternInt32x8, OpternInt32x16,
		OpternInt64x2, OpternInt64x4, OpternInt64x8,
		OpternUint32x4, OpternUint32x8, OpternUint32x16,
		OpternUint64x2, OpternUint64x4, OpternUint64x8,
		OpbitSelectInt8x16, OpbitSelectNotInt8x16,
		OpblendInt8x16, OpblendInt8x32,
	} {
		t.Run(op.String(), func(t *testing.T) {
			info, _ := goALLCSIMDInfo(op)
			laneBits := int(info.laneBits)
			width := 128
			switch op {
			case OpternInt32x8, OpternInt64x4, OpternUint32x8, OpternUint64x4, OpblendInt8x32:
				width = 256
			case OpternInt32x16, OpternInt64x8, OpternUint32x16, OpternUint64x8:
				width = 512
			}
			lanes := width / laneBits
			carrier := map[int]*types.Type{128: types.TypeVec128, 256: types.TypeVec256, 512: types.TypeVec512}[width]
			arch := "amd64"
			if op == OpbitSelectInt8x16 || op == OpbitSelectNotInt8x16 {
				arch = "arm64"
			}
			module := GlobalCtxt.NewModule("logic")
			CurrentModule = module
			builder := GlobalCtxt.NewBuilder()
			defer module.Dispose()
			defer builder.Dispose()
			vec := llvm.VectorType(GlobalCtxt.IntType(laneBits), lanes)
			fn := llvm.AddFunction(module, "logic", llvm.FunctionType(vec, nil, false))
			builder.SetInsertPointAtEnd(llvm.AddBasicBlock(fn, "entry"))
			args := []*Value{{ID: 1, Op: OpArg, Type: carrier}, {ID: 2, Op: OpArg, Type: carrier}, {ID: 3, Op: OpArg, Type: carrier}}
			values := [3][]uint64{}
			mapped := map[ID]llvm.Value{}
			for j, pattern := range []uint64{0xf0f0f0f0f0f0f0f0, 0xcccccccccccccccc, 0xaaaaaaaaaaaaaaaa} {
				values[j] = make([]uint64, lanes)
				constants := make([]llvm.Value, lanes)
				for i := range constants {
					values[j][i] = bits.RotateLeft64(pattern, i) & (^uint64(0) >> (64 - laneBits))
					constants[i] = llvm.ConstInt(GlobalCtxt.IntType(laneBits), values[j][i], false)
				}
				mapped[ID(j+1)] = llvm.ConstBitCast(llvm.ConstVector(constants, false), getLLVMType(carrier))
			}
			var result llvm.Value
			for table := range 256 {
				if info.lowering != goALLCSIMDLowerTernary {
					masks := make([]llvm.Value, lanes)
					for i := range masks {
						values[2][i] = uint64(uint8(table + i))
						masks[i] = llvm.ConstInt(GlobalCtxt.Int8Type(), values[2][i], false)
					}
					mapped[3] = llvm.ConstBitCast(llvm.ConstVector(masks, false), getLLVMType(carrier))
				}
				ctx := &LLVMFuncContext{
					F:           &Func{Config: &Config{arch: arch}, Entry: &Block{CPUfeatures: CPUavx | CPUavx2 | CPUavx512}},
					CPUFeatures: &llvmCPUFeaturePlan{floor: info.archInfo(arch).cpuProfile},
					Vs:          mapped, b: builder,
				}
				v := &Value{ID: 4, Op: op, Type: carrier, Args: args, AuxInt: int64(int8(table))}
				delete(mapped, v.ID)
				result = ctx.GenLV(v)
				for i := range lanes {
					x, y, z := values[0][i], values[1][i], values[2][i]
					var want uint64
					switch op {
					case OpbitSelectInt8x16:
						want = x&z | y&^z
					case OpbitSelectNotInt8x16:
						want = y&z | x&^z
					case OpblendInt8x16, OpblendInt8x32:
						want = x
						if z&128 != 0 {
							want = y
						}
					default:
						for bit := range laneBits {
							index := ((x>>bit)&1)<<2 | ((y>>bit)&1)<<1 | (z>>bit)&1
							want |= uint64(table>>index&1) << bit
						}
					}
					got := llvm.ConstExtractElement(result, llvm.ConstInt(GlobalCtxt.Int32Type(), uint64(i), false))
					if got.IsAConstantInt().IsNil() || got.ZExtValue() != want {
						t.Fatalf("table=%#x lane=%d got=%s want=%#x", table, i, got.String(), want)
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
