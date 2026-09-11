// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/types"
	"fmt"
	"strings"
	"testing"

	"github.com/goallc/go-llvm"
)

func TestLLVMGeneratedSIMDSaturatingConversions(t *testing.T) {
	oldTypes, oldModule := type2lTypes, CurrentModule
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() { type2lTypes, CurrentModule = oldTypes, oldModule }()
	tests := []struct {
		op                                        Op
		inBits, inLanes, outBits, outLanes, arity int
		signedIn, signedOut                       bool
	}{
		{OpSaturateToInt8Int16x8, 16, 8, 8, 16, 1, true, true},
		{OpSaturateToInt8Int16x16, 16, 16, 8, 16, 1, true, true},
		{OpSaturateToInt8Int16x32, 16, 32, 8, 32, 1, true, true},
		{OpSaturateToInt8Int32x4, 32, 4, 8, 16, 1, true, true},
		{OpSaturateToInt8Int32x8, 32, 8, 8, 16, 1, true, true},
		{OpSaturateToInt8Int32x16, 32, 16, 8, 16, 1, true, true},
		{OpSaturateToInt8Int64x2, 64, 2, 8, 16, 1, true, true},
		{OpSaturateToInt8Int64x4, 64, 4, 8, 16, 1, true, true},
		{OpSaturateToInt8Int64x8, 64, 8, 8, 16, 1, true, true},
		{OpSaturateToInt16Int32x4, 32, 4, 16, 8, 1, true, true},
		{OpSaturateToInt16Int32x8, 32, 8, 16, 8, 1, true, true},
		{OpSaturateToInt16Int32x16, 32, 16, 16, 16, 1, true, true},
		{OpSaturateToInt16Int64x2, 64, 2, 16, 8, 1, true, true},
		{OpSaturateToInt16Int64x4, 64, 4, 16, 8, 1, true, true},
		{OpSaturateToInt16Int64x8, 64, 8, 16, 8, 1, true, true},
		{OpSaturateToInt16ConcatInt32x4, 32, 4, 16, 8, 2, true, true},
		{OpSaturateToInt16ConcatGroupedInt32x8, 32, 8, 16, 16, 2, true, true},
		{OpSaturateToInt16ConcatGroupedInt32x16, 32, 16, 16, 32, 2, true, true},
		{OpSaturateToInt32Int64x2, 64, 2, 32, 4, 1, true, true},
		{OpSaturateToInt32Int64x4, 64, 4, 32, 4, 1, true, true},
		{OpSaturateToInt32Int64x8, 64, 8, 32, 8, 1, true, true},
		{OpSaturateToUint8Uint16x8, 16, 8, 8, 16, 1, false, false},
		{OpSaturateToUint8Uint16x16, 16, 16, 8, 16, 1, false, false},
		{OpSaturateToUint8Uint16x32, 16, 32, 8, 32, 1, false, false},
		{OpSaturateToUint8Uint32x4, 32, 4, 8, 16, 1, false, false},
		{OpSaturateToUint8Uint32x8, 32, 8, 8, 16, 1, false, false},
		{OpSaturateToUint8Uint32x16, 32, 16, 8, 16, 1, false, false},
		{OpSaturateToUint8Uint64x2, 64, 2, 8, 16, 1, false, false},
		{OpSaturateToUint8Uint64x4, 64, 4, 8, 16, 1, false, false},
		{OpSaturateToUint8Uint64x8, 64, 8, 8, 16, 1, false, false},
		{OpSaturateToUint16Uint32x4, 32, 4, 16, 8, 1, false, false},
		{OpSaturateToUint16Uint32x8, 32, 8, 16, 8, 1, false, false},
		{OpSaturateToUint16Uint32x16, 32, 16, 16, 16, 1, false, false},
		{OpSaturateToUint16Uint64x2, 64, 2, 16, 8, 1, false, false},
		{OpSaturateToUint16Uint64x4, 64, 4, 16, 8, 1, false, false},
		{OpSaturateToUint16Uint64x8, 64, 8, 16, 8, 1, false, false},
		{OpSaturateToUint16ConcatInt32x4, 32, 4, 16, 8, 2, true, false},
		{OpSaturateToUint16ConcatGroupedInt32x8, 32, 8, 16, 16, 2, true, false},
		{OpSaturateToUint16ConcatGroupedInt32x16, 32, 16, 16, 32, 2, true, false},
		{OpSaturateToUint32Uint64x2, 64, 2, 32, 4, 1, false, false},
		{OpSaturateToUint32Uint64x4, 64, 4, 32, 4, 1, false, false},
		{OpSaturateToUint32Uint64x8, 64, 8, 32, 8, 1, false, false},
		{OpSaturateToUint8Int16x8, 16, 8, 8, 16, 1, true, false},
		{OpSaturateToUint16Int32x4, 32, 4, 16, 8, 1, true, false},
		{OpSaturateToUint32Int64x2, 64, 2, 32, 4, 1, true, false},
	}
	seen := make(map[Op]bool)
	for _, test := range tests {
		seen[test.op] = true
		for _, carrier := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/carrier=%v", test.op, carrier), func(t *testing.T) {
				elem := func(bits int, signed bool) *types.Type {
					kind := map[int]types.Kind{8: types.TUINT8, 16: types.TUINT16, 32: types.TUINT32, 64: types.TUINT64}[bits]
					if signed {
						kind = map[int]types.Kind{8: types.TINT8, 16: types.TINT16, 32: types.TINT32, 64: types.TINT64}[bits]
					}
					return types.Types[kind]
				}
				input := llvmTestSIMDType("saturation-input", elem(test.inBits, test.signedIn), int64(test.inLanes))
				output := llvmTestSIMDType("saturation-output", elem(test.outBits, test.signedOut), int64(test.outLanes))
				resultType := getLLVMType(output)
				if carrier {
					vec := func(bits int) *types.Type {
						return map[int]*types.Type{128: types.TypeVec128, 256: types.TypeVec256, 512: types.TypeVec512}[bits]
					}
					input, output = vec(test.inBits*test.inLanes), vec(test.outBits*test.outLanes)
				}
				module := GlobalCtxt.NewModule("saturation")
				CurrentModule = module
				builder := GlobalCtxt.NewBuilder()
				defer module.Dispose()
				defer builder.Dispose()
				params := make([]llvm.Type, test.arity)
				for i := range params {
					params[i] = getLLVMType(input)
				}
				function := llvm.AddFunction(module, "saturate", llvm.FunctionType(resultType, params, false))
				builder.SetInsertPointAtEnd(llvm.AddBasicBlock(function, "entry"))
				context := &LLVMFuncContext{
					F:  &Func{Config: &Config{arch: "amd64"}, Entry: &Block{CPUfeatures: CPUavx | CPUavx2 | CPUavx512}},
					Vs: make(map[ID]llvm.Value), b: builder,
				}
				args := make([]*Value, test.arity)
				for i := range args {
					args[i] = &Value{ID: ID(i + 1), Op: OpArg, Type: input}
					context.Vs[args[i].ID] = function.Param(i)
				}
				v := &Value{ID: 3, Op: test.op, Type: output, Args: args}
				builder.CreateRet(context.GenLV(v))
				if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
					t.Fatalf("invalid saturating conversion IR: %v\n%s", err, module.String())
				}
				ir := module.String()
				operations := []string{"umin"}
				if test.signedIn {
					operations = []string{"smax", "smin"}
				}
				for _, op := range operations {
					want := fmt.Sprintf("call <%d x i%d> @llvm.%s.v%di%d", test.inLanes, test.inBits, op, test.inLanes, test.inBits)
					if got := strings.Count(ir, want); got != test.arity {
						t.Errorf("got %d occurrences of %q, want %d\n%s", got, want, test.arity, ir)
					}
				}
				if !strings.Contains(ir, fmt.Sprintf("trunc <%d x i%d>", test.inLanes, test.inBits)) ||
					!strings.Contains(ir, fmt.Sprintf("to <%d x i%d>", test.inLanes, test.outBits)) {
					t.Errorf("missing narrowing cast\n%s", ir)
				}
				high := uint64(1)<<test.outBits - 1
				var low int64
				if test.signedOut {
					high >>= 1
					low = -int64(high) - 1
				}
				if !strings.Contains(ir, fmt.Sprintf("i%d %d", test.inBits, high)) {
					t.Errorf("missing upper saturation bound %d\n%s", high, ir)
				}
				if test.signedIn && low != 0 && !strings.Contains(ir, fmt.Sprintf("i%d %d", test.inBits, low)) {
					t.Errorf("missing lower saturation bound %d\n%s", low, ir)
				}
				if test.signedIn && !test.signedOut && !strings.Contains(ir, "zeroinitializer") {
					t.Errorf("signed-to-unsigned conversion must clamp negatives to zero\n%s", ir)
				}
				if test.arity == 1 && test.outLanes > test.inLanes &&
					(!strings.Contains(ir, "shufflevector") || !strings.Contains(ir, "zeroinitializer")) {
					t.Errorf("missing zero-filled high result lanes\n%s", ir)
				}
				if test.arity == 2 {
					// Literal masks make the 128-bit grouping independent of the lowering's
					// mask-construction algorithm.
					masks := map[int][]int{
						4:  {0, 1, 2, 3, 4, 5, 6, 7},
						8:  {0, 1, 2, 3, 8, 9, 10, 11, 4, 5, 6, 7, 12, 13, 14, 15},
						16: {0, 1, 2, 3, 16, 17, 18, 19, 4, 5, 6, 7, 20, 21, 22, 23, 8, 9, 10, 11, 24, 25, 26, 27, 12, 13, 14, 15, 28, 29, 30, 31},
					}
					var elements []string
					for _, index := range masks[test.inLanes] {
						elements = append(elements, fmt.Sprintf("i32 %d", index))
					}
					if !strings.Contains(ir, "<"+strings.Join(elements, ", ")+">") {
						t.Errorf("wrong 128-bit pack order\n%s", ir)
					}
				}
			})
		}
	}
	// Adding a new generated saturation descriptor also requires an explicit
	// destination shape and source-kind test above.
	for op := Op(0); int(op) < len(goALLCSIMDOpcodeIndex); op++ {
		info, ok := goALLCSIMDInfo(op)
		if ok && (info.lowering == goALLCSIMDLowerSaturateInteger || info.lowering == goALLCSIMDLowerSaturateIntegerPack128) && !seen[op] {
			t.Errorf("missing generated saturation test for %s", op)
		}
	}
}
