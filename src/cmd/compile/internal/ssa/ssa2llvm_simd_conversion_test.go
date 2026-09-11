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

func TestLLVMGeneratedSIMDIntegerConversions(t *testing.T) {
	oldTypes, oldModule := type2lTypes, CurrentModule
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() { type2lTypes, CurrentModule = oldTypes, oldModule }()
	for _, test := range []struct {
		op                                 Op
		inBits, inLanes, outBits, outLanes int
		cast                               string
	}{
		{OpExtendLo8ToInt16Int8x16, 8, 16, 16, 8, "sext"},
		{OpExtendLo4ToUint32Uint8x16, 8, 16, 32, 4, "zext"},
		{OpExtendLo2ToInt64Int16x8, 16, 8, 64, 2, "sext"},
		{OpExtendLo4ToUint64Uint8x16, 8, 16, 64, 4, "zext"},
		{OpExtendLo8ToInt64Int8x16, 8, 16, 64, 8, "sext"},
		{OpExtendToInt16Int8x16, 8, 16, 16, 16, "sext"},
		{OpExtendToUint32Uint16x16, 16, 16, 32, 16, "zext"},
		{OpTruncToInt8Int64x2, 64, 2, 8, 16, "trunc"},
		{OpTruncToUint16Uint32x4, 32, 4, 16, 8, "trunc"},
		{OpTruncToInt32Int64x8, 64, 8, 32, 8, "trunc"},
		{OpTruncToUint8Uint16x32, 16, 32, 8, 32, "trunc"},
	} {
		for _, carrier := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/carrier=%v", test.op, carrier), func(t *testing.T) {
				elem := func(bits int) *types.Type {
					return types.Types[map[int]types.Kind{8: types.TINT8, 16: types.TINT16, 32: types.TINT32, 64: types.TINT64}[bits]]
				}
				input := llvmTestSIMDType("conversion-input", elem(test.inBits), int64(test.inLanes))
				output := llvmTestSIMDType("conversion-output", elem(test.outBits), int64(test.outLanes))
				// Internal TypeVec values may materialize with natural lanes; the
				// source-level return boundary uses the destination lane shape.
				resultType := getLLVMType(output)
				if carrier {
					vec := func(bits int) *types.Type {
						return map[int]*types.Type{128: types.TypeVec128, 256: types.TypeVec256, 512: types.TypeVec512}[bits]
					}
					input, output = vec(test.inBits*test.inLanes), vec(test.outBits*test.outLanes)
				}
				module := GlobalCtxt.NewModule("conversion")
				CurrentModule = module
				builder := GlobalCtxt.NewBuilder()
				defer module.Dispose()
				defer builder.Dispose()
				function := llvm.AddFunction(module, "convert", llvm.FunctionType(resultType, []llvm.Type{getLLVMType(input)}, false))
				builder.SetInsertPointAtEnd(llvm.AddBasicBlock(function, "entry"))
				context := &LLVMFuncContext{
					F:  &Func{Config: &Config{arch: "amd64"}, Entry: &Block{CPUfeatures: CPUavx | CPUavx2 | CPUavx512}},
					Vs: make(map[ID]llvm.Value), b: builder,
				}
				x := &Value{ID: 1, Op: OpArg, Type: input}
				context.Vs[x.ID] = function.Param(0)
				v := &Value{ID: 2, Op: test.op, Type: output, Args: []*Value{x}}
				builder.CreateRet(context.GenLV(v))
				if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
					t.Fatalf("invalid conversion IR: %v\n%s", err, module.String())
				}
				ir := module.String()
				lanes := min(test.inLanes, test.outLanes)
				for _, want := range []string{
					fmt.Sprintf("%s <%d x i%d>", test.cast, lanes, test.inBits),
					fmt.Sprintf("to <%d x i%d>", lanes, test.outBits),
				} {
					if !strings.Contains(ir, want) {
						t.Errorf("missing %q\n%s", want, ir)
					}
				}
				if test.inLanes != test.outLanes && !strings.Contains(ir, "shufflevector") {
					t.Errorf("missing lane selection/padding\n%s", ir)
				}
				if test.outLanes > test.inLanes && !strings.Contains(ir, "zeroinitializer") {
					t.Errorf("truncation must zero high lanes\n%s", ir)
				}
			})
		}
	}
}
