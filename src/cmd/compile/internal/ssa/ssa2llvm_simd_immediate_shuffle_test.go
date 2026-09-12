// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/types"
	"fmt"
	"github.com/goallc/go-llvm"
	"strings"
	"testing"
)

// Reference routing builds source groups explicitly, independently of the
// lowering's per-output mask arithmetic.
func immediateShuffleReference(family string, bits, n, control int) []uint64 {
	x, y := make([]uint64, n), make([]uint64, n)
	for i := range x {
		x[i], y[i] = uint64(i+1), uint64(n+i+1)
	}
	out := append([]uint64(nil), x...)
	group := 128 / bits
	if family == "ConcatPermute128Scalars" {
		sources := append(x, y...)
		for half := 0; half < 2; half++ {
			c := control >> (4 * half)
			if c&8 != 0 {
				clear(out[half*group : (half+1)*group])
				continue
			}
			copy(out[half*group:], sources[(c&3)*group:((c&3)+1)*group])
		}
		return out
	}
	for base := 0; base < n; base += group {
		left, right := x[base:base+group], y[base:base+group]
		switch {
		case strings.HasPrefix(family, "ConcatShiftBytesRight"):
			src := append(append(append([]uint64(nil), right...), left...), make([]uint64, 256+group)...)
			copy(out[base:base+group], src[control:control+group])
		case strings.HasPrefix(family, "concatSelectedConstant"):
			selectors := control
			if bits == 64 {
				selectors >>= base
			}
			field, mask := 2, 3
			if bits == 64 {
				field, mask = 1, 1
			}
			for j := 0; j < group; j++ {
				src := left
				if j >= group/2 {
					src = right
				}
				out[base+j] = src[selectors&mask]
				selectors >>= field
			}
		default:
			offset := 0
			if strings.Contains(family, "Hi") {
				offset = 4
			}
			selectors := control
			for j := 0; j < 4; j++ {
				out[base+offset+j] = left[offset+(selectors&3)]
				selectors >>= 2
			}
		}
	}
	return out
}

func TestLLVMGeneratedSIMDImmediateShuffles(t *testing.T) {
	oldTypes, oldModule := type2lTypes, CurrentModule
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() { type2lTypes, CurrentModule = oldTypes, oldModule }()
	tests := []struct {
		op                 Op
		family, kind       string
		bits, lanes, arity int
		arches             string
	}{
		{OpConcatPermute128ScalarsFloat32x8, "ConcatPermute128Scalars", "float", 32, 8, 2, "amd64"},
		{OpConcatPermute128ScalarsFloat64x4, "ConcatPermute128Scalars", "float", 64, 4, 2, "amd64"},
		{OpConcatPermute128ScalarsInt8x32, "ConcatPermute128Scalars", "int", 8, 32, 2, "amd64"},
		{OpConcatPermute128ScalarsInt16x16, "ConcatPermute128Scalars", "int", 16, 16, 2, "amd64"},
		{OpConcatPermute128ScalarsInt32x8, "ConcatPermute128Scalars", "int", 32, 8, 2, "amd64"},
		{OpConcatPermute128ScalarsInt64x4, "ConcatPermute128Scalars", "int", 64, 4, 2, "amd64"},
		{OpConcatPermute128ScalarsUint8x32, "ConcatPermute128Scalars", "uint", 8, 32, 2, "amd64"},
		{OpConcatPermute128ScalarsUint16x16, "ConcatPermute128Scalars", "uint", 16, 16, 2, "amd64"},
		{OpConcatPermute128ScalarsUint32x8, "ConcatPermute128Scalars", "uint", 32, 8, 2, "amd64"},
		{OpConcatPermute128ScalarsUint64x4, "ConcatPermute128Scalars", "uint", 64, 4, 2, "amd64"},
		{OpConcatShiftBytesRightGroupedUint8x32, "ConcatShiftBytesRightGrouped", "uint", 8, 32, 2, "amd64"},
		{OpConcatShiftBytesRightGroupedUint8x64, "ConcatShiftBytesRightGrouped", "uint", 8, 64, 2, "amd64"},
		{OpConcatShiftBytesRightUint8x16, "ConcatShiftBytesRight", "uint", 8, 16, 2, "amd64,arm64"},
		{OpconcatSelectedConstantFloat32x4, "concatSelectedConstant", "float", 32, 4, 2, "amd64"},
		{OpconcatSelectedConstantFloat64x2, "concatSelectedConstant", "float", 64, 2, 2, "amd64"},
		{OpconcatSelectedConstantGroupedFloat32x8, "concatSelectedConstantGrouped", "float", 32, 8, 2, "amd64"},
		{OpconcatSelectedConstantGroupedFloat32x16, "concatSelectedConstantGrouped", "float", 32, 16, 2, "amd64"},
		{OpconcatSelectedConstantGroupedFloat64x4, "concatSelectedConstantGrouped", "float", 64, 4, 2, "amd64"},
		{OpconcatSelectedConstantGroupedFloat64x8, "concatSelectedConstantGrouped", "float", 64, 8, 2, "amd64"},
		{OpconcatSelectedConstantGroupedInt32x8, "concatSelectedConstantGrouped", "int", 32, 8, 2, "amd64"},
		{OpconcatSelectedConstantGroupedInt32x16, "concatSelectedConstantGrouped", "int", 32, 16, 2, "amd64"},
		{OpconcatSelectedConstantGroupedInt64x4, "concatSelectedConstantGrouped", "int", 64, 4, 2, "amd64"},
		{OpconcatSelectedConstantGroupedInt64x8, "concatSelectedConstantGrouped", "int", 64, 8, 2, "amd64"},
		{OpconcatSelectedConstantGroupedUint32x8, "concatSelectedConstantGrouped", "uint", 32, 8, 2, "amd64"},
		{OpconcatSelectedConstantGroupedUint32x16, "concatSelectedConstantGrouped", "uint", 32, 16, 2, "amd64"},
		{OpconcatSelectedConstantGroupedUint64x4, "concatSelectedConstantGrouped", "uint", 64, 4, 2, "amd64"},
		{OpconcatSelectedConstantGroupedUint64x8, "concatSelectedConstantGrouped", "uint", 64, 8, 2, "amd64"},
		{OpconcatSelectedConstantInt32x4, "concatSelectedConstant", "int", 32, 4, 2, "amd64"},
		{OpconcatSelectedConstantInt64x2, "concatSelectedConstant", "int", 64, 2, 2, "amd64"},
		{OpconcatSelectedConstantUint32x4, "concatSelectedConstant", "uint", 32, 4, 2, "amd64"},
		{OpconcatSelectedConstantUint64x2, "concatSelectedConstant", "uint", 64, 2, 2, "amd64"},
		{OppermuteScalarsGroupedInt32x8, "permuteScalarsGrouped", "int", 32, 8, 1, "amd64"},
		{OppermuteScalarsGroupedInt32x16, "permuteScalarsGrouped", "int", 32, 16, 1, "amd64"},
		{OppermuteScalarsGroupedUint32x8, "permuteScalarsGrouped", "uint", 32, 8, 1, "amd64"},
		{OppermuteScalarsGroupedUint32x16, "permuteScalarsGrouped", "uint", 32, 16, 1, "amd64"},
		{OppermuteScalarsHiGroupedInt16x16, "permuteScalarsHiGrouped", "int", 16, 16, 1, "amd64"},
		{OppermuteScalarsHiGroupedInt16x32, "permuteScalarsHiGrouped", "int", 16, 32, 1, "amd64"},
		{OppermuteScalarsHiGroupedUint16x16, "permuteScalarsHiGrouped", "uint", 16, 16, 1, "amd64"},
		{OppermuteScalarsHiGroupedUint16x32, "permuteScalarsHiGrouped", "uint", 16, 32, 1, "amd64"},
		{OppermuteScalarsHiInt16x8, "permuteScalarsHi", "int", 16, 8, 1, "amd64"},
		{OppermuteScalarsHiUint16x8, "permuteScalarsHi", "uint", 16, 8, 1, "amd64"},
		{OppermuteScalarsInt32x4, "permuteScalars", "int", 32, 4, 1, "amd64"},
		{OppermuteScalarsLoGroupedInt16x16, "permuteScalarsLoGrouped", "int", 16, 16, 1, "amd64"},
		{OppermuteScalarsLoGroupedInt16x32, "permuteScalarsLoGrouped", "int", 16, 32, 1, "amd64"},
		{OppermuteScalarsLoGroupedUint16x16, "permuteScalarsLoGrouped", "uint", 16, 16, 1, "amd64"},
		{OppermuteScalarsLoGroupedUint16x32, "permuteScalarsLoGrouped", "uint", 16, 32, 1, "amd64"},
		{OppermuteScalarsLoInt16x8, "permuteScalarsLo", "int", 16, 8, 1, "amd64"},
		{OppermuteScalarsLoUint16x8, "permuteScalarsLo", "uint", 16, 8, 1, "amd64"},
		{OppermuteScalarsUint32x4, "permuteScalars", "uint", 32, 4, 1, "amd64"},
	}
	seen := make(map[Op]bool)
	for _, test := range tests {
		seen[test.op] = true
		for _, arch := range strings.Split(test.arches, ",") {
			for _, carrier := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/carrier=%v", test.op, arch, carrier), func(t *testing.T) {
					kinds := map[string]map[int]types.Kind{
						"int":   {8: types.TINT8, 16: types.TINT16, 32: types.TINT32, 64: types.TINT64},
						"uint":  {8: types.TUINT8, 16: types.TUINT16, 32: types.TUINT32, 64: types.TUINT64},
						"float": {32: types.TFLOAT32, 64: types.TFLOAT64},
					}
					vector := llvmTestSIMDType("immediate-vector", types.Types[kinds[test.kind][test.bits]], int64(test.lanes))
					resultType := getLLVMType(vector)
					if carrier {
						vector = map[int]*types.Type{128: types.TypeVec128, 256: types.TypeVec256, 512: types.TypeVec512}[test.bits*test.lanes]
					}
					typ := getLLVMType(vector)
					integer := GlobalCtxt.IntType(test.bits)
					integerVector := llvm.VectorType(integer, test.lanes)
					module := GlobalCtxt.NewModule("immediate-shuffle")
					CurrentModule = module
					builder := GlobalCtxt.NewBuilder()
					defer module.Dispose()
					defer builder.Dispose()
					max := 256
					if arch == "arm64" {
						max = 16
					}
					for control := 0; control < max; control++ {
						fn := llvm.AddFunction(module, fmt.Sprintf("shuffle%d", control), llvm.FunctionType(resultType, nil, false))
						builder.SetInsertPointAtEnd(llvm.AddBasicBlock(fn, "entry"))
						ctx := &LLVMFuncContext{F: &Func{Config: &Config{arch: arch}, Entry: &Block{CPUfeatures: CPUavx | CPUavx2 | CPUavx512}}, Vs: make(map[ID]llvm.Value), b: builder}
						args := make([]*Value, test.arity)
						for side := range args {
							args[side] = &Value{ID: ID(side + 1), Op: OpArg, Type: vector}
							values := make([]llvm.Value, test.lanes)
							for i := range values {
								values[i] = llvm.ConstInt(integer, uint64(side*test.lanes+i+1), false)
							}
							ctx.Vs[args[side].ID] = llvm.ConstBitCast(llvm.ConstVector(values, false), typ)
						}
						v := &Value{ID: 3, Op: test.op, Type: vector, Args: args, AuxInt: int64(int8(control))}
						result := ctx.GenLV(v)
						builder.CreateRet(result)
						result = llvm.ConstBitCast(result, integerVector)
						want := immediateShuffleReference(test.family, test.bits, test.lanes, control)
						for i, expected := range want {
							got := llvm.ConstExtractElement(result, llvm.ConstInt(GlobalCtxt.Int32Type(), uint64(i), false))
							if got.IsAConstantInt().IsNil() || got.ZExtValue() != expected {
								t.Fatalf("control=%#x lane=%d got=%s want=%d", control, i, got.String(), expected)
							}
						}
					}
					if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
						t.Fatal(err)
					}
					ir := module.String()
					for _, forbidden := range []string{" poison", " undef", "@llvm.x86.", "@llvm.aarch64."} {
						if strings.Contains(ir, forbidden) {
							t.Fatalf("unexpected %s in IR", forbidden)
						}
					}
				})
			}
		}
	}
	for op := Op(0); int(op) < len(goALLCSIMDOpcodeIndex); op++ {
		info, ok := goALLCSIMDInfo(op)
		if ok && info.lowering >= goALLCSIMDLowerPermute32_128 && info.lowering <= goALLCSIMDLowerConcatShiftBytes128 && !seen[op] {
			t.Errorf("missing immediate shuffle test for %s", op)
		}
	}
}
